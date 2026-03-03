package detector

// DBType 标识数据库类型
type DBType string

const (
	DBTypeGreenplum DBType = "greenplum"
	DBTypePostgres  DBType = "postgres"
	DBTypeOpenGauss DBType = "opengauss"
	DBTypeUnknown   DBType = "unknown"
)

// NodeInfo 保存数据库节点的关键信息
type NodeInfo struct {
	Host    string // 节点 IP 或主机名
	Port    int    // 数据库端口
	Role    string // master / segment / primary / standby / coordinator / datanode
	DataDir string // 数据目录（pg_log 的父级）
	DBType  DBType // 所属数据库类型
}

// DBAdapter 统一适配器接口
type DBAdapter interface {
	// Detect 探测数据库类型（通过 version() 或 SSH 命令）
	Detect() (DBType, error)
	// DiscoverNodes 发现所有节点（master/segment/primary/standby 等）
	DiscoverNodes() ([]NodeInfo, error)
	// GetLogPaths 返回指定节点的日志目录路径列表
	GetLogPaths(node NodeInfo) []string
}
