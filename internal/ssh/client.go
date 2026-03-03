package ssh

import (
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbpacklogs/internal/config"
	"dbpacklogs/pkg/utils"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	maxRetries     = 3
	baseRetryDelay = time.Second
	dialTimeout    = 30 * time.Second
	cmdTimeout     = 5 * time.Minute // SSH 命令执行超时 5 分钟
)

// SSHClient 封装单个 SSH 连接
type SSHClient struct {
	client *ssh.Client
	Host   string
}

// HostConfig 封装建立连接所需的主机参数
type HostConfig struct {
	Host     string
	Port     int
	User     string
	KeyPath  string
	Password string
	// 跳过主机密钥校验（不安全）
	InsecureHostKey bool
}

func (h *HostConfig) addr() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

func (h *HostConfig) sshClientConfig() *ssh.ClientConfig {
	log := utils.GetLogger()

	// 尝试从 known_hosts 加载主机密钥回调
	knownHostsPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if h.InsecureHostKey {
		log.Warnf("已启用 --insecure-hostkey，跳过主机密钥校验（不安全）")
	} else {
		callback, err := knownhosts.New(knownHostsPath)
		if err != nil {
			log.Warnf("解析 known_hosts 失败 (%s): %v，将拒绝未知主机（可使用 --insecure-hostkey 跳过）", knownHostsPath, err)
			hostKeyCallback = func(hostname string, _ net.Addr, _ ssh.PublicKey) error {
				return fmt.Errorf("known_hosts 不可用，拒绝连接到 %s（如需跳过校验请使用 --insecure-hostkey）", hostname)
			}
		} else {
			hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				if err := callback(hostname, remote, key); err != nil {
					if keyErr, ok := err.(*knownhosts.KeyError); ok {
						if len(keyErr.Want) == 0 {
							return fmt.Errorf("主机 %s 不在 known_hosts 中（可使用 --insecure-hostkey 跳过）", hostname)
						}
						return fmt.Errorf("主机 %s 的密钥与 known_hosts 不匹配: %w", hostname, err)
					}
					return err
				}
				return nil
			}
		}
	}

	return &ssh.ClientConfig{
		User:            h.User,
		Auth:            buildAuthMethods(h.KeyPath, h.Password),
		HostKeyCallback: hostKeyCallback,
		Timeout:         dialTimeout,
	}
}

// buildAuthMethods 构建认证方法列表：
// 1) 若显式指定 --ssh-key，优先使用该私钥；
// 2) 否则自动尝试默认私钥（~/.ssh/id_rsa、~/.ssh/id_ed25519）；
// 3) 最后降级到密码认证（如果提供）。
func buildAuthMethods(keyPath string, password string) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// 1. 尝试密钥认证
	for _, kp := range candidateKeyPaths(keyPath) {
		key, err := os.ReadFile(kp)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			utils.GetLogger().Warnf("解析私钥 %s 失败: %v", kp, err)
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
		break
	}

	// 2. 降级到密码认证
	if password != "" {
		methods = append(methods, ssh.Password(password))
	}

	return methods
}

func candidateKeyPaths(explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
	}
}

// Connect 建立 SSH 直连，指数退避重试 maxRetries 次
func Connect(host string, port int, cfg *config.Config) (*SSHClient, error) {
	hc := &HostConfig{
		Host:            host,
		Port:            port,
		User:            cfg.SSHUser,
		KeyPath:         cfg.SSHKey, // 若为空则自动读取默认密钥 ~/.ssh/id_rsa, ~/.ssh/id_ed25519
		Password:        cfg.SSHPassword,
		InsecureHostKey: cfg.InsecureHostKey,
	}
	return connectWithRetry(hc)
}

// connectWithRetry 使用指数退避策略重试 SSH 连接（1s→2s→4s）
// 认证失败不重试，网络错误才重试
func connectWithRetry(hc *HostConfig) (*SSHClient, error) {
	log := utils.GetLogger()
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			delay := time.Duration(math.Pow(2, float64(i-1))) * baseRetryDelay
			log.Warnf("SSH 连接 %s 失败（第 %d 次），%.0fs 后重试: %v", hc.addr(), i, delay.Seconds(), lastErr)
			time.Sleep(delay)
		}

		client, err := ssh.Dial("tcp", hc.addr(), hc.sshClientConfig())
		if err == nil {
			return &SSHClient{client: client, Host: hc.Host}, nil
		}

		lastErr = err

		// 认证失败不重试
		if isAuthError(err) {
			log.Errorf("SSH 认证失败，不再重试: %v", err)
			break
		}

		// 检查是否是临时网络错误
		if !isTemporaryError(err) {
			log.Errorf("SSH 连接遇到永久性错误，不再重试: %v", err)
			break
		}
	}

	return nil, fmt.Errorf("SSH 连接 %s 失败（已重试 %d 次）: %w", hc.addr(), maxRetries, lastErr)
}

// isAuthError 判断是否是认证错误
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "auth") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "no supported methods")
}

// isTemporaryError 判断是否是临时网络错误
func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// 超时、连接拒绝、临时不可用等
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "i/o timeout") {
		return true
	}
	// 兜底：error 链中包含 *net.OpError 类型（如 ECONNRESET 等）
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// Execute 在远端执行命令，返回 stdout 字节内容
// 使用超时避免命令无限等待
func (c *SSHClient) Execute(cmd string) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	// 设置命令执行超时
	done := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		output, err := session.Output(cmd)
		if err != nil {
			errCh <- err
			return
		}
		done <- output
	}()

	select {
	case output := <-done:
		return output, nil
	case err := <-errCh:
		return nil, fmt.Errorf("命令执行失败: %w", err)
	case <-time.After(cmdTimeout):
		session.Signal(ssh.Signal("KILL"))
		return nil, fmt.Errorf("命令执行超时（%s）", cmdTimeout)
	}
}

// NewSession 创建新的 SSH session（供调用方自行管理 Stdout/Stdin）
func (c *SSHClient) NewSession() (*ssh.Session, error) {
	return c.client.NewSession()
}

// UnderlyingClient 返回底层 *ssh.Client（供 SFTP 使用）
func (c *SSHClient) UnderlyingClient() *ssh.Client {
	return c.client
}

// IsAlive 发送一个空请求检测连接是否仍然存活
func (c *SSHClient) IsAlive() bool {
	_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		utils.GetLogger().Debugf("SSH keepalive 检测失败: %v", err)
	}
	return err == nil
}

// Close 关闭 SSH 连接
func (c *SSHClient) Close() error {
	return c.client.Close()
}
