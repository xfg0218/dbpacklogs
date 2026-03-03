package ssh

import (
	"fmt"
	"sync"

	"dbpacklogs/internal/config"
	"dbpacklogs/pkg/utils"
)

// Pool 管理多节点 SSH 连接复用，线程安全
type Pool struct {
	mu    sync.Mutex
	conns map[string]*SSHClient
	cfg   *config.Config
}

// NewPool 创建连接池
func NewPool(cfg *config.Config) *Pool {
	return &Pool{
		conns: make(map[string]*SSHClient),
		cfg:   cfg,
	}
}

// Get 获取指定节点的 SSH 连接。
// 优先复用缓存连接，复用前通过 keepalive 检测存活；若已断开则重建连接。
func (p *Pool) Get(host string, port int) (*SSHClient, error) {
	key := fmt.Sprintf("%s:%d", host, port)
	log := utils.GetLogger()

	p.mu.Lock()
	if c, ok := p.conns[key]; ok {
		// 检测连接是否仍然存活
		if c.IsAlive() {
			p.mu.Unlock()
			return c, nil
		}
		// 连接已断开，从缓存中清理并重建
		log.Warnf("[pool] 节点 %s 连接已失效，重新建立", key)
		_ = c.Close()
		delete(p.conns, key)
	}
	p.mu.Unlock()

	// 在锁外建立连接，避免阻塞其他节点
	c, err := Connect(host, port, p.cfg)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	// 双检锁：避免并发建立重复连接
	if existing, ok := p.conns[key]; ok {
		p.mu.Unlock()
		_ = c.Close()
		return existing, nil
	}
	p.conns[key] = c
	p.mu.Unlock()
	return c, nil
}

// CloseAll 关闭连接池中所有连接
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, c := range p.conns {
		_ = c.Close()
		delete(p.conns, key)
	}
}
