package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbpacklogs/internal/config"
	"dbpacklogs/internal/detector"
	"dbpacklogs/internal/filter"
	intssh "dbpacklogs/internal/ssh"
	"dbpacklogs/pkg/utils"

	"github.com/jackc/pgx/v5"
)

// DBCollector 负责收集数据库配置文件和日志
type DBCollector struct {
	cfg     *config.Config
	adapter detector.DBAdapter
}

// NewDBCollector 创建 DBCollector
func NewDBCollector(cfg *config.Config, adapter detector.DBAdapter) *DBCollector {
	return &DBCollector{cfg: cfg, adapter: adapter}
}

// CollectInfo 收集数据库配置信息（postgresql.conf、pg_hba.conf、集群拓扑）
// 保存到 dbInfoDir 目录下。调用前应先通过 EnsureDataDir 填充 node.DataDir。
func (c *DBCollector) CollectInfo(sshClient *intssh.SSHClient, node detector.NodeInfo, dbInfoDir string) error {
	log := utils.GetLogger()
	if err := os.MkdirAll(dbInfoDir, 0755); err != nil {
		return fmt.Errorf("创建 db_info 目录失败: %w", err)
	}

	skipConfigs := false
	if node.DataDir == "" {
		log.Warnf("[%s] data_directory 为空，跳过配置文件收集", node.Host)
		skipConfigs = true
	}

	// 根据数据库类型确定配置文件路径
	confPaths := c.getConfigFilePaths(node)

	// 下载配置文件
	if !skipConfigs {
		for _, cp := range confPaths {
			remotePath := filepath.Join(node.DataDir, cp.remote)
			localPath := filepath.Join(dbInfoDir, cp.local)
			if err := intssh.DownloadFile(sshClient, remotePath, localPath); err != nil {
				log.Warnf("[%s] 下载 %s 失败: %v", node.Host, cp.remote, err)
			}
		}
	}

	// 收集集群拓扑信息（仅在关键节点执行）
	if shouldCollectTopology(node) {
		if err := c.collectTopology(node, dbInfoDir); err != nil {
			log.Warnf("[%s] 收集集群拓扑失败: %v", node.Host, err)
		}
	}
	return nil
}

type configPath struct {
	remote string // 远端文件名
	local  string // 本地保存的文件名
}

// getConfigFilePaths 根据数据库类型返回需要收集的配置文件列表
func (c *DBCollector) getConfigFilePaths(node detector.NodeInfo) []configPath {
	switch node.DBType {
	case detector.DBTypeGreenplum:
		return []configPath{
			{"postgresql.conf", "postgresql.conf"},
			{"pg_hba.conf", "pg_hba.conf"},
			{"pg_ident.conf", "pg_ident.conf"},
		}
	case detector.DBTypeOpenGauss:
		return []configPath{
			{"postgresql.conf", "postgresql.conf"},
			{"pg_hba.conf", "pg_hba.conf"},
		}
	default: // PostgreSQL
		return []configPath{
			{"postgresql.conf", "postgresql.conf"},
			{"pg_hba.conf", "pg_hba.conf"},
		}
	}
}

// queryDataDir 通过 SSH 查询节点的 data_directory
func (c *DBCollector) queryDataDir(sshClient *intssh.SSHClient, node detector.NodeInfo) (string, error) {
	// 使用单引号包裹参数，避免命令注入
	cmd := fmt.Sprintf("psql -h '%s' -p %d -U '%s' -d '%s' -t -A -c \"SELECT current_setting('data_directory')\" 2>/dev/null || echo ''",
		node.Host, node.Port, c.cfg.DBUser, c.cfg.DBName)
	out, err := sshClient.Execute(cmd)
	if err != nil {
		return "", fmt.Errorf("查询失败: %w", err)
	}

	dataDir := strings.TrimSpace(string(out))
	if dataDir == "" {
		return "", fmt.Errorf("未获取到 data_directory")
	}

	// 验证返回的是有效路径
	if !strings.HasPrefix(dataDir, "/") {
		return "", fmt.Errorf("无效的 data_directory: %s", dataDir)
	}

	return dataDir, nil
}

// EnsureDataDir 尝试补齐节点 DataDir（仅适用于 PostgreSQL/Greenplum）。
// 接收指针以便将查询结果直接写回调用方持有的 node 变量，避免重复查询。
func (c *DBCollector) EnsureDataDir(sshClient *intssh.SSHClient, node *detector.NodeInfo) {
	if node.DataDir != "" {
		return
	}
	if node.DBType != detector.DBTypePostgres && node.DBType != detector.DBTypeGreenplum {
		return
	}
	dataDir, err := c.queryDataDir(sshClient, *node)
	if err != nil {
		utils.GetLogger().Warnf("[%s] 查询 data_directory 失败: %v", node.Host, err)
		return
	}
	node.DataDir = dataDir
}

// shouldCollectTopology 判断是否需要收集拓扑信息
func shouldCollectTopology(node detector.NodeInfo) bool {
	switch node.DBType {
	case detector.DBTypeGreenplum:
		return node.Role == "coordinator" || node.Role == "master"
	case detector.DBTypePostgres:
		return node.Role == "primary"
	default:
		return false
	}
}

// collectTopology 根据数据库类型收集集群拓扑信息
func (c *DBCollector) collectTopology(node detector.NodeInfo, dbInfoDir string) error {
	var query string
	switch node.DBType {
	case detector.DBTypeGreenplum:
		query = "SELECT dbid, content, role, preferred_role, mode, status, address, port, datadir FROM gp_segment_configuration ORDER BY content"
	case detector.DBTypePostgres:
		query = "SELECT pid, usename, application_name, client_addr, state, sent_lsn, write_lsn, flush_lsn, replay_lsn FROM pg_stat_replication"
	default:
		return nil
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		node.Host, node.Port, c.cfg.DBUser, c.cfg.DBPassword, c.cfg.DBName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	outFile := filepath.Join(dbInfoDir, "cluster_topology.txt")
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fieldDescs := rows.FieldDescriptions()
	for i, fd := range fieldDescs {
		if i > 0 {
			fmt.Fprint(f, "\t")
		}
		fmt.Fprint(f, string(fd.Name))
	}
	fmt.Fprintln(f)
	fmt.Fprintln(f, "---")

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			continue
		}
		for i, v := range values {
			if i > 0 {
				fmt.Fprint(f, "\t")
			}
			fmt.Fprintf(f, "%v", v)
		}
		fmt.Fprintln(f)
	}
	return rows.Err()
}

// safeHostName 将 host 中的点、冒号替换为下划线，确保文件名安全
func safeHostName(host string) string {
	r := strings.NewReplacer(".", "_", ":", "_")
	return r.Replace(host)
}

// isValidRemotePath 验证远端路径安全性
// 必须是绝对路径，不能包含引号、管道、分号等特殊字符
func isValidRemotePath(path string) bool {
	if path == "" {
		return false
	}
	// 必须是绝对路径
	if path[0] != '/' {
		return false
	}
	// 检查不安全字符
	dangerousChars := []string{"'", "\"", "`", ";", "|", "&", "$", "(", ")", "{", "}", "[", "]", "<", ">", "\n", "\r"}
	for _, c := range dangerousChars {
		if strings.Contains(path, c) {
			return false
		}
	}
	return true
}

// CollectLogs 按时间范围收集节点日志，流式下载到 dbLogsDir。
// 调用前应先通过 EnsureDataDir 填充 node.DataDir。
func (c *DBCollector) CollectLogs(sshClient *intssh.SSHClient, node detector.NodeInfo, dbLogsDir string, tf *filter.TimeFilter) error {
	log := utils.GetLogger()
	if err := os.MkdirAll(dbLogsDir, 0755); err != nil {
		return fmt.Errorf("创建 db_logs 目录失败: %w", err)
	}

	if node.DataDir == "" {
		log.Warnf("[%s] data_directory 为空，跳过日志收集", node.Host)
		return nil
	}

	logPaths := c.adapter.GetLogPaths(node)
	if len(logPaths) == 0 {
		log.Warnf("[%s] 适配器未返回有效日志路径", node.Host)
		return nil
	}

	for _, logDir := range logPaths {
		// 验证路径安全性：必须是绝对路径，不能包含特殊字符
		if !isValidRemotePath(logDir) {
			log.Warnf("[%s] 路径 %s 包含不安全字符，跳过", node.Host, logDir)
			continue
		}

		// 检查目录是否存在
		checkCmd := fmt.Sprintf("test -d '%s' && echo 'exists' || echo 'not_exists'", logDir)
		out, err := sshClient.Execute(checkCmd)
		if err != nil || strings.Contains(string(out), "not_exists") {
			log.Debugf("[%s] 日志目录 %s 不存在，跳过", node.Host, logDir)
			continue
		}

		files, err := intssh.FindRemoteFiles(sshClient, logDir, tf.FindArgs())
		if err != nil {
			log.Warnf("[%s] find 日志文件失败（%s）: %v", node.Host, logDir, err)
			continue
		}
		if len(files) == 0 {
			log.Infof("[%s] 日志目录 %s 在指定时间范围内无文件", node.Host, logDir)
			continue
		}

		log.Infof("[%s] 找到 %d 个日志文件，开始流式传输", node.Host, len(files))

		// 使用日志目录名作为文件名的一部分，避免覆盖
		logDirName := filepath.Base(logDir)
		outFile := filepath.Join(dbLogsDir, fmt.Sprintf("pg_log_%s_%s.tar.gz", safeHostName(node.Host), logDirName))
		f, err := os.Create(outFile)
		if err != nil {
			log.Warnf("[%s] 创建本地输出文件失败: %v，跳过日志目录 %s", node.Host, err, logDir)
			continue
		}

		if err := intssh.RemoteCompress(sshClient, files, f); err != nil {
			_ = f.Close()
			// 删除因传输失败而残留的空文件，避免遗留无效压缩包
			_ = os.Remove(outFile)
			log.Warnf("[%s] 流式传输日志失败: %v", node.Host, err)
			continue
		}
		_ = f.Close()
		log.Infof("[%s] 日志传输完成: %s", node.Host, outFile)
	}
	return nil
}
