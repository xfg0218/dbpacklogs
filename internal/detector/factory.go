package detector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbpacklogs/internal/config"
	intssh "dbpacklogs/internal/ssh"
	"dbpacklogs/pkg/utils"

	"github.com/jackc/pgx/v5"
)

const dbConnectTimeout = 10 * time.Second

// NewAdapter 工厂函数：通过 SELECT version() 自动路由到对应适配器。
// cfg.DBHost 由 config.Validate() 从 --hosts 第一个节点自动填充，无需手动指定。
// 数据库连接失败时返回明确错误，不自动回退到其他数据库类型。
func NewAdapter(cfg *config.Config, sshPool *intssh.Pool) (DBAdapter, error) {
	log := utils.GetLogger()

	// 尝试通过数据库连接探测类型（DBHost = Hosts[0]，由 Validate() 保证非空）
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=10",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), dbConnectTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("数据库连接 %s:%d 失败: %w，请检查 --hosts/--all-hosts、--db-port、--db-user、--db-password 参数是否正确", cfg.DBHost, cfg.DBPort, err)
	}
	defer conn.Close(context.Background())

	var version string
	if err := conn.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return nil, fmt.Errorf("执行 version() 失败: %w", err)
	}
	log.Debugf("数据库 version() 返回: %s", version)

	versionLower := strings.ToLower(version)
	switch {
	case strings.Contains(versionLower, "greenplum"):
		log.Infof("检测到数据库类型: Greenplum")
		return NewGreenplumAdapter(cfg), nil
	case strings.Contains(versionLower, "opengauss"), strings.Contains(versionLower, "gaussdb"):
		log.Infof("检测到数据库类型: openGauss")
		sshClient, sshErr := sshPool.Get(cfg.DBHost, cfg.SSHPort)
		if sshErr != nil {
			return nil, fmt.Errorf("openGauss SSH 连接失败: %w", sshErr)
		}
		return NewOpenGaussAdapter(sshClient, cfg), nil
	case strings.Contains(versionLower, "postgresql"):
		log.Infof("检测到数据库类型: PostgreSQL")
		return NewPostgresAdapter(cfg), nil
	default:
		return nil, fmt.Errorf("无法识别的数据库类型: %s，当前工具仅支持 Greenplum、PostgreSQL、openGauss", version)
	}
}
