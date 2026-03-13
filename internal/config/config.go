package config

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
)

// Config 保存 CLI 解析后的所有配置参数
type Config struct {
	// 节点参数
	AllHosts bool
	Hosts    []string // --hosts 解析后的列表

	// SSH 参数
	SSHPort     int
	SSHUser     string
	SSHKey      string
	SSHPassword string
	// 跳过主机密钥校验（不安全）
	InsecureHostKey bool

	// 输出参数
	Output   string
	PackType string // zip | tar

	// 时间范围
	StartTime string
	EndTime   string

	// 数据库连接参数（DBHost 由 Hosts[0] 自动推导，无需手动指定）
	DBHost     string // 内部使用，由 Validate() 自动填充
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// 运行参数
	Verbose bool
}

// ParseHosts 将逗号分隔的 hosts 字符串解析为列表
func ParseHosts(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ParseEtcHosts 读取 /etc/hosts 文件，解析出所有IP地址
// 过滤掉 localhost、ipv6 localhost、广播地址等
func ParseEtcHosts() ([]string, error) {
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return nil, err
	}

	var ips []string
	seen := make(map[string]struct{})
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// /etc/hosts 格式：第一列是 IP，后续列是主机名
		ip := fields[0]
		// 跳过 ipv6
		if strings.Contains(ip, ":") {
			continue
		}
		// 跳过非 IP 地址
		if !IsIP(ip) {
			continue
		}
		// 过滤掉 localhost、广播地址
		if ip == "127.0.0.1" || ip == "0.0.0.0" || ip == "255.255.255.255" {
			continue
		}
		// 去重
		if _, exists := seen[ip]; exists {
			continue
		}
		seen[ip] = struct{}{}
		ips = append(ips, ip)
	}
	return ips, nil
}

// IsIP 判断字符串是否为有效的 IPv4 地址
func IsIP(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		// 验证每个字段是否为数字且在 0-255 范围内
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
		// 转换为数字验证范围
		n := 0
		for _, c := range part {
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}

// BuildDSN 构建 PostgreSQL/Greenplum/openGauss 连接字符串（libpq key=value 格式）。
// connect_timeout 单位为秒，传 0 时不追加该参数（使用默认值）。
func (c *Config) BuildDSN(host string, port int, connectTimeoutSec int) string {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, c.DBUser, c.DBPassword, c.DBName)
	if connectTimeoutSec > 0 {
		dsn += fmt.Sprintf(" connect_timeout=%d", connectTimeoutSec)
	}
	return dsn
}

// Validate 纯粹校验配置字段合法性，不产生任何副作用（不修改配置）。
// 调用前应先调用 Initialize() 完成自动推导，否则 DBHost/SSHUser 等字段可能为空。
func (c *Config) Validate() error {
	if c.AllHosts && len(c.Hosts) > 0 {
		return errors.New("--all-hosts 与 --hosts 互斥，不能同时指定")
	}
	if !c.AllHosts && len(c.Hosts) == 0 {
		return errors.New("必须指定 --hosts 或 --all-hosts")
	}
	if c.SSHUser == "" {
		return errors.New("SSHUser 未初始化，请先调用 Initialize()")
	}
	if c.PackType != "zip" && c.PackType != "tar" {
		return errors.New("--pack-type 只支持 zip 或 tar")
	}
	return nil
}

// Initialize 自动推导并填充配置默认值（有副作用），然后调用 Validate() 执行纯粹校验。
// 调用方应使用此方法代替直接调用 Validate()。
// 内部执行的操作：
//   - 若未指定 --ssh-user，从当前 OS 用户推导
//   - 若未指定 --db-user，与 SSHUser 保持一致
//   - 若指定 --all-hosts，读取 /etc/hosts 填充 Hosts
//   - 若 DBHost 为空，从 Hosts[0] 自动推导
func (c *Config) Initialize() error {
	if c.AllHosts && len(c.Hosts) > 0 {
		return errors.New("--all-hosts 与 --hosts 互斥，不能同时指定")
	}
	if !c.AllHosts && len(c.Hosts) == 0 {
		return errors.New("必须指定 --hosts 或 --all-hosts")
	}
	// SSHUser 默认为当前操作系统用户
	if c.SSHUser == "" {
		u, err := user.Current()
		if err != nil {
			return errors.New("获取当前系统用户失败，请通过 --ssh-user 手动指定")
		}
		c.SSHUser = u.Username
	}
	// DBUser 默认与 SSHUser 保持一致（master 节点通常使用 peer 认证）
	if c.DBUser == "" {
		c.DBUser = c.SSHUser
	}
	if c.AllHosts {
		hosts, err := ParseEtcHosts()
		if err != nil {
			return errors.New("读取 /etc/hosts 失败，请检查文件权限或是否存在")
		}
		if len(hosts) == 0 {
			return errors.New("/etc/hosts 中未找到有效IP")
		}
		c.Hosts = hosts
	}
	if c.DBHost == "" && len(c.Hosts) > 0 {
		c.DBHost = c.Hosts[0]
	}
	if c.PackType != "zip" && c.PackType != "tar" {
		return errors.New("--pack-type 只支持 zip 或 tar")
	}
	return nil
}
