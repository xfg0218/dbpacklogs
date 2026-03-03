package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"dbpacklogs/internal/config"

	"github.com/jackc/pgx/v5"
)

// PostgresAdapter 适配标准 PostgreSQL
type PostgresAdapter struct {
	cfg *config.Config
}

// NewPostgresAdapter 创建 PostgreSQL 适配器
func NewPostgresAdapter(cfg *config.Config) *PostgresAdapter {
	return &PostgresAdapter{cfg: cfg}
}

// Detect 返回 DBTypePostgres
func (a *PostgresAdapter) Detect() (DBType, error) {
	return DBTypePostgres, nil
}

// DiscoverNodes 查询 current_setting('data_directory')，返回单节点。
// 若部署了流复制（pg_stat_replication），同时追加 standby 节点。
// 连接地址使用 cfg.DBHost（由 config.Validate() 从 --hosts 第一个节点自动推导）。
func (a *PostgresAdapter) DiscoverNodes() ([]NodeInfo, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		a.cfg.DBHost, a.cfg.DBPort, a.cfg.DBUser, a.cfg.DBPassword, a.cfg.DBName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQL 连接失败: %w", err)
	}
	defer conn.Close(context.Background())

	// 获取主节点数据目录
	var dataDir string
	if err := conn.QueryRow(ctx,
		"SELECT current_setting('data_directory')").Scan(&dataDir); err != nil {
		return nil, fmt.Errorf("获取 data_directory 失败: %w", err)
	}

	primary := NodeInfo{
		Host:    a.cfg.DBHost,
		Port:    a.cfg.DBPort,
		Role:    "primary",
		DataDir: dataDir,
		DBType:  DBTypePostgres,
	}
	nodes := []NodeInfo{primary}

	// 尝试查询流复制 standby 节点（忽略失败，可能无复制配置）
	rows, err := conn.Query(ctx,
		"SELECT client_addr, application_name FROM pg_stat_replication WHERE state = 'streaming'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var clientAddr, appName *string
			if err := rows.Scan(&clientAddr, &appName); err != nil || clientAddr == nil {
				continue
			}
			standbyHost := *clientAddr
			// 尝试获取 standby 的 data_directory（通过 pg_settings）
			// 注意：standby 可能无法直接查询，需要通过 SSH
			nodes = append(nodes, NodeInfo{
				Host:    standbyHost,
				Port:    a.cfg.DBPort,
				Role:    "standby",
				DataDir: "", // 将在收集阶段通过 SSH 查询
				DBType:  DBTypePostgres,
			})
		}
	}
	return nodes, nil
}

// GetLogPaths 返回 PostgreSQL 节点的 pg_log 目录路径
// PostgreSQL 12+ 可能使用 log_directory 配置，优先使用 pg_log
func (a *PostgresAdapter) GetLogPaths(node NodeInfo) []string {
	if node.DataDir == "" {
		return nil
	}
	paths := []string{
		filepath.Join(node.DataDir, "pg_log"),
		filepath.Join(node.DataDir, "log"),
	}
	return paths
}
