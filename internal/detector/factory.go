package detector

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"dbpacklogs/internal/config"
	intssh "dbpacklogs/internal/ssh"
	"dbpacklogs/pkg/utils"

	"github.com/jackc/pgx/v5"
)

// DBConnectTimeout 统一的数据库连接超时时长，供所有适配器共用
const DBConnectTimeout = 30 * time.Second

// dbConnectTimeoutSec 是 DSN connect_timeout 参数（秒），与 DBConnectTimeout 保持一致
const dbConnectTimeoutSec = 30

// versionPatterns 定义数据库类型匹配的正则表达式，按优先级排序
// Greenplum 和 openGauss 基于 PostgreSQL，需要优先匹配
var versionPatterns = []struct {
	pattern *regexp.Regexp
	dbType  string
}{
	{regexp.MustCompile(`(?i)greenplum`), "Greenplum"},
	{regexp.MustCompile(`(?i)(opengauss|gaussdb)`), "openGauss"},
	{regexp.MustCompile(`(?i)postgresql`), "PostgreSQL"},
}

// detectDBType 从 version 字符串中探测数据库类型
// 使用优先级匹配，避免误判（如 Greenplum 也包含 PostgreSQL 字样）
func detectDBType(version string) (string, error) {
	for _, vp := range versionPatterns {
		if vp.pattern.MatchString(version) {
			return vp.dbType, nil
		}
	}
	return "", fmt.Errorf("无法识别的数据库类型：%s，当前工具仅支持 Greenplum、PostgreSQL、openGauss", version)
}

// NewAdapter 工厂函数：通过 SELECT version() 自动路由到对应适配器。
// cfg.DBHost 由 config.Initialize() 从 --hosts 第一个节点自动填充，无需手动指定。
// 数据库连接失败时返回明确错误，不自动回退到其他数据库类型。
func NewAdapter(cfg *config.Config, sshPool *intssh.Pool) (DBAdapter, error) {
	log := utils.GetLogger()

	// 尝试通过数据库连接探测类型（DBHost = Hosts[0]，由 Initialize() 保证非空）
	dsn := cfg.BuildDSN(cfg.DBHost, cfg.DBPort, dbConnectTimeoutSec)

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), DBConnectTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("数据库连接 %s:%d 失败：%w，请检查 --hosts/--all-hosts、--db-port、--db-user、--db-password 参数是否正确", cfg.DBHost, cfg.DBPort, err)
	}
	defer conn.Close(context.Background())

	var version string
	if err := conn.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return nil, fmt.Errorf("执行 version() 失败：%w", err)
	}
	log.Debugf("数据库 version() 返回：%s", version)

	// 使用优先级匹配探测数据库类型
	dbType, err := detectDBType(version)
	if err != nil {
		return nil, err
	}
	log.Infof("检测到数据库类型：%s", dbType)

	switch dbType {
	case "Greenplum":
		return NewGreenplumAdapter(cfg), nil
	case "openGauss":
		sshClient, sshErr := sshPool.Get(cfg.DBHost, cfg.SSHPort)
		if sshErr != nil {
			return nil, fmt.Errorf("openGauss SSH 连接失败：%w", sshErr)
		}
		return NewOpenGaussAdapter(sshClient, cfg), nil
	case "PostgreSQL":
		return NewPostgresAdapter(cfg), nil
	default:
		// 理论上不会到达这里，但保留防御性代码
		return nil, fmt.Errorf("不支持的数据库类型：%s", dbType)
	}
}
