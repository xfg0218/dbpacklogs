package detector

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"dbpacklogs/internal/config"
	intssh "dbpacklogs/internal/ssh"
	"dbpacklogs/pkg/utils"
)

// OpenGaussAdapter 适配 openGauss（通过 SSH 执行 cm_ctl）
type OpenGaussAdapter struct {
	sshClient *intssh.SSHClient
	cfg       *config.Config
}

// NewOpenGaussAdapter 创建 openGauss 适配器
func NewOpenGaussAdapter(sshClient *intssh.SSHClient, cfg *config.Config) *OpenGaussAdapter {
	return &OpenGaussAdapter{sshClient: sshClient, cfg: cfg}
}

// Detect 返回 DBTypeOpenGauss
func (a *OpenGaussAdapter) Detect() (DBType, error) {
	return DBTypeOpenGauss, nil
}

// DiscoverNodes 通过 SSH 执行 cm_ctl query -Cv 解析集群拓扑
func (a *OpenGaussAdapter) DiscoverNodes() ([]NodeInfo, error) {
	log := utils.GetLogger()

	// 尝试使用 cm_ctl 查询集群拓扑
	out, err := a.sshClient.Execute("cm_ctl query -Cv 2>/dev/null")
	if err != nil {
		// 尝试 gs_om
		out, err = a.sshClient.Execute("gs_om -t status --detail 2>/dev/null")
		if err != nil {
			log.Warnf("cm_ctl / gs_om 执行失败: %v，尝试单节点模式", err)
			return a.discoverSingleNode()
		}
		nodes := parseGsOmOutput(string(out), a.cfg)
		if len(nodes) == 0 {
			log.Warnf("gs_om 输出解析结果为空，尝试单节点模式")
			return a.discoverSingleNode()
		}
		return nodes, nil
	}

	nodes := parseCmCtlOutput(string(out), a.cfg)
	if len(nodes) == 0 {
		log.Warnf("cm_ctl 输出解析结果为空，尝试单节点模式")
		return a.discoverSingleNode()
	}
	return nodes, nil
}

// discoverSingleNode 单节点模式：通过 gsql 获取 data_directory。
// cfg.DBHost 由 config.Validate() 从 --hosts 第一个节点自动推导。
func (a *OpenGaussAdapter) discoverSingleNode() ([]NodeInfo, error) {
	// 验证输入参数
	if a.cfg.DBHost == "" {
		return nil, fmt.Errorf("DBHost 未配置")
	}

	// 使用单引号包裹参数，避免注入
	cmd := fmt.Sprintf(
		`gsql -h '%s' -p %d -U '%s' -d '%s' -c "SELECT current_setting('data_directory')" -t -A 2>/dev/null || echo ''`,
		a.cfg.DBHost, a.cfg.DBPort, a.cfg.DBUser, a.cfg.DBName,
	)
	out, err := a.sshClient.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("openGauss 单节点探测失败: %w", err)
	}
	dataDir := strings.TrimSpace(string(out))
	if dataDir == "" {
		return nil, fmt.Errorf("openGauss 返回空 data_directory")
	}

	// 验证返回的是有效路径
	if !strings.HasPrefix(dataDir, "/") {
		return nil, fmt.Errorf("无效的 data_directory: %s", dataDir)
	}

	return []NodeInfo{{
		Host:    a.cfg.DBHost,
		Port:    a.cfg.DBPort,
		Role:    "primary",
		DataDir: dataDir,
		DBType:  DBTypeOpenGauss,
	}}, nil
}

// parseCmCtlOutput 解析 cm_ctl query -Cv 输出，提取节点信息
// 典型输出片段：
//
//	NodeIndex:   1
//	HostName:    node1
//	IP:          10.0.0.10
//	DataPath:    /data/gauss/dn1
//	InstanceRole: Primary
var (
	reNodeIndex = regexp.MustCompile(`(?i)NodeIndex:\s*(\d+)`)
	reIP        = regexp.MustCompile(`(?i)(?:IP|Address):\s*(\S+)`)
	reDataPath  = regexp.MustCompile(`(?i)DataPath:\s*(\S+)`)
	reRole      = regexp.MustCompile(`(?i)InstanceRole:\s*(\S+)`)
)

func parseCmCtlOutput(output string, cfg *config.Config) []NodeInfo {
	log := utils.GetLogger()
	var nodes []NodeInfo

	if output == "" {
		log.Warnf("cm_ctl 输出为空，无法解析节点信息")
		return nodes
	}

	// 按节点块分割（以 NodeIndex 为分隔符）
	blocks := reNodeIndex.Split(output, -1)
	if len(blocks) <= 1 {
		log.Warnf("cm_ctl 输出格式不符合预期，未找到节点信息")
		return nodes
	}

	for _, block := range blocks[1:] { // 跳过第一个空块
		var ip, dataPath, role string

		if m := reIP.FindStringSubmatch(block); m != nil {
			ip = m[1]
		}
		if m := reDataPath.FindStringSubmatch(block); m != nil {
			dataPath = m[1]
		}
		if m := reRole.FindStringSubmatch(block); m != nil {
			role = strings.ToLower(m[1])
		}

		if ip == "" {
			continue
		}
		nodes = append(nodes, NodeInfo{
			Host:    ip,
			Port:    cfg.DBPort,
			Role:    role,
			DataDir: dataPath,
			DBType:  DBTypeOpenGauss,
		})
	}
	return nodes
}

// parseGsOmOutput 解析 gs_om -t status --detail 输出，提取节点信息
// 典型输出片段：
//
//	cluster_state   : Normal
//	redistributing  : No
//	balancer        :
//	------------------------------ nodes information ------------------------------
//	|  node_name  |  node_ip  |  role  |  port  |   data_path   |
//	|     node1   | 10.0.0.10 |  main  | 5432   | /data/gauss/dn1 |
var (
	reGsOmNodeName = regexp.MustCompile(`(?i)\|\s*node_name\s*\|\s*node_ip\s*\|\s*role\s*\|\s*port\s*\|\s*data_path`)
	reGsOmData     = regexp.MustCompile(`\|\s*(\S+)\s*\|\s*(\S+)\s*\|\s*(\S+)\s*\|\s*(\d+)\s*\|\s*(\S+)\s*\|`)
)

func parseGsOmOutput(output string, cfg *config.Config) []NodeInfo {
	var nodes []NodeInfo

	// 检查是否有节点信息表头
	if !reGsOmNodeName.MatchString(output) {
		return nodes
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		m := reGsOmData.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		nodeName := m[1]
		ip := m[2]
		role := strings.ToLower(m[3])
		dataPath := m[5]

		// 跳过表头行
		if nodeName == "node_name" || ip == "node_ip" {
			continue
		}

		// role: main -> primary, standby -> standby
		if role == "main" {
			role = "primary"
		}

		nodes = append(nodes, NodeInfo{
			Host:    ip,
			Port:    cfg.DBPort,
			Role:    role,
			DataDir: dataPath,
			DBType:  DBTypeOpenGauss,
		})
	}
	return nodes
}

// GetLogPaths 返回 openGauss 节点的日志目录路径
func (a *OpenGaussAdapter) GetLogPaths(node NodeInfo) []string {
	if node.DataDir == "" {
		return nil
	}
	// openGauss 日志通常在 pg_log 或 logs 子目录下
	return []string{
		filepath.Join(node.DataDir, "pg_log"),
		filepath.Join(node.DataDir, "logs"),
	}
}
