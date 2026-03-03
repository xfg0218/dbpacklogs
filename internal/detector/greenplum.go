package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"dbpacklogs/internal/config"
	"dbpacklogs/pkg/utils"

	"github.com/jackc/pgx/v5"
)

// GreenplumAdapter 适配 Greenplum 数据库
type GreenplumAdapter struct {
	cfg *config.Config
}

// NewGreenplumAdapter 创建 Greenplum 适配器
func NewGreenplumAdapter(cfg *config.Config) *GreenplumAdapter {
	return &GreenplumAdapter{cfg: cfg}
}

// Detect 返回 DBTypeGreenplum
func (a *GreenplumAdapter) Detect() (DBType, error) {
	return DBTypeGreenplum, nil
}

// DiscoverNodes 查询 gp_segment_configuration 获取所有节点。
// 连接地址使用 cfg.DBHost（由 config.Validate() 从 --hosts 第一个节点自动推导）。
func (a *GreenplumAdapter) DiscoverNodes() ([]NodeInfo, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		a.cfg.DBHost, a.cfg.DBPort, a.cfg.DBUser, a.cfg.DBPassword, a.cfg.DBName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("Greenplum 连接失败: %w", err)
	}
	defer conn.Close(context.Background())

	// 先获取所有 coordinator/master 节点
	coordinatorRows, err := conn.Query(ctx,
		"SELECT address, port, datadir FROM gp_segment_configuration WHERE content = -1 AND status = 'u'")

	coordinators := make(map[string]bool)
	if err == nil {
		defer coordinatorRows.Close()
		for coordinatorRows.Next() {
			var addr, dataDir string
			var port int
			if err := coordinatorRows.Scan(&addr, &port, &dataDir); err == nil {
				coordinators[addr] = true
			}
		}
	} else {
		// coordinator 查询失败，记录警告但继续执行
		utils.GetLogger().Warnf("查询 coordinator 节点失败，将使用默认角色: %v", err)
	}

	// 查询所有节点（master/coordinator + primary/standby + mirror）
	rows, err := conn.Query(ctx, `
		SELECT address, port, role, preferred_role, datadir
		FROM gp_segment_configuration
		WHERE status = 'u'
		ORDER BY content, role
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 gp_segment_configuration 失败: %w", err)
	}
	defer rows.Close()

	var nodes []NodeInfo
	for rows.Next() {
		var (
			address       string
			port          int
			role          string
			preferredRole string
			dataDir       string
		)
		if err := rows.Scan(&address, &port, &role, &preferredRole, &dataDir); err != nil {
			continue
		}

		roleName := "segment"
		// 检查是否为 coordinator/master 节点
		if coordinators[address] {
			roleName = "coordinator"
		} else if role == "p" && preferredRole == "p" {
			roleName = "primary"
		} else if role == "m" && preferredRole == "m" {
			roleName = "mirror"
		} else if role == "m" && preferredRole == "p" {
			roleName = "standby"
		} else if role == "p" && preferredRole == "m" {
			// primary 变为 mirror 的情况
			roleName = "primary_to_mirror"
		}

		nodes = append(nodes, NodeInfo{
			Host:    address,
			Port:    port,
			Role:    roleName,
			DataDir: dataDir,
			DBType:  DBTypeGreenplum,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 gp_segment_configuration 结果集失败: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("gp_segment_configuration 返回空结果，数据库可能未正常运行")
	}
	return nodes, nil
}

// GetLogPaths 返回 Greenplum 节点的 pg_log 目录路径
func (a *GreenplumAdapter) GetLogPaths(node NodeInfo) []string {
	return []string{filepath.Join(node.DataDir, "pg_log")}
}
