package detector

import (
	"testing"

	"dbpacklogs/internal/config"
)

// ---- parseCmCtlOutput ----

func TestParseCmCtlOutput_Normal(t *testing.T) {
	output := `
NodeIndex:   1
HostName:    node1
IP:          10.0.0.10
DataPath:    /data/gauss/dn1
InstanceRole: Primary

NodeIndex:   2
HostName:    node2
IP:          10.0.0.11
DataPath:    /data/gauss/dn2
InstanceRole: Standby
`
	cfg := &config.Config{DBPort: 5432}
	nodes := parseCmCtlOutput(output, cfg)

	if len(nodes) != 2 {
		t.Fatalf("parseCmCtlOutput() got %d nodes, want 2", len(nodes))
	}
	if nodes[0].Host != "10.0.0.10" {
		t.Errorf("nodes[0].Host = %q, want '10.0.0.10'", nodes[0].Host)
	}
	if nodes[0].Role != "primary" {
		t.Errorf("nodes[0].Role = %q, want 'primary'", nodes[0].Role)
	}
	if nodes[0].DataDir != "/data/gauss/dn1" {
		t.Errorf("nodes[0].DataDir = %q, want '/data/gauss/dn1'", nodes[0].DataDir)
	}
	if nodes[0].DBType != DBTypeOpenGauss {
		t.Errorf("nodes[0].DBType = %q, want 'opengauss'", nodes[0].DBType)
	}
	if nodes[1].Host != "10.0.0.11" {
		t.Errorf("nodes[1].Host = %q, want '10.0.0.11'", nodes[1].Host)
	}
	if nodes[1].Role != "standby" {
		t.Errorf("nodes[1].Role = %q, want 'standby'", nodes[1].Role)
	}
}

func TestParseCmCtlOutput_Empty(t *testing.T) {
	cfg := &config.Config{DBPort: 5432}
	nodes := parseCmCtlOutput("", cfg)
	if len(nodes) != 0 {
		t.Errorf("parseCmCtlOutput('') = %d nodes, want 0", len(nodes))
	}
}

func TestParseCmCtlOutput_NoNodeIndex(t *testing.T) {
	output := "some random output without NodeIndex"
	cfg := &config.Config{DBPort: 5432}
	nodes := parseCmCtlOutput(output, cfg)
	if len(nodes) != 0 {
		t.Errorf("parseCmCtlOutput() = %d nodes, want 0", len(nodes))
	}
}

func TestParseCmCtlOutput_MissingIP(t *testing.T) {
	// 缺少 IP 字段的块应被跳过
	output := `
NodeIndex:   1
DataPath:    /data/gauss/dn1
InstanceRole: Primary
`
	cfg := &config.Config{DBPort: 5432}
	nodes := parseCmCtlOutput(output, cfg)
	if len(nodes) != 0 {
		t.Errorf("parseCmCtlOutput() should skip block without IP, got %d", len(nodes))
	}
}

func TestParseCmCtlOutput_AddressAlias(t *testing.T) {
	// 使用 Address 而非 IP（大小写不敏感）
	output := `
NodeIndex:   1
Address:     10.0.0.20
DataPath:    /data/gauss/dn1
InstanceRole: Primary
`
	cfg := &config.Config{DBPort: 5432}
	nodes := parseCmCtlOutput(output, cfg)
	if len(nodes) != 1 {
		t.Fatalf("parseCmCtlOutput() got %d nodes, want 1", len(nodes))
	}
	if nodes[0].Host != "10.0.0.20" {
		t.Errorf("Host = %q, want '10.0.0.20'", nodes[0].Host)
	}
}

func TestParseCmCtlOutput_RoleLowercase(t *testing.T) {
	// InstanceRole 应被转换为小写
	output := `
NodeIndex:   1
IP:          10.0.0.10
DataPath:    /data/dn1
InstanceRole: PRIMARY
`
	cfg := &config.Config{DBPort: 5432}
	nodes := parseCmCtlOutput(output, cfg)
	if len(nodes) != 1 {
		t.Fatalf("parseCmCtlOutput() got %d nodes, want 1", len(nodes))
	}
	if nodes[0].Role != "primary" {
		t.Errorf("Role = %q, want 'primary' (lowercase)", nodes[0].Role)
	}
}

func TestParseCmCtlOutput_DBPortFromConfig(t *testing.T) {
	output := `
NodeIndex:   1
IP:          10.0.0.10
DataPath:    /data/dn1
InstanceRole: Primary
`
	cfg := &config.Config{DBPort: 2345}
	nodes := parseCmCtlOutput(output, cfg)
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	if nodes[0].Port != 2345 {
		t.Errorf("Port = %d, want 2345", nodes[0].Port)
	}
}

// ---- parseGsOmOutput ----

func TestParseGsOmOutput_Normal(t *testing.T) {
	output := `cluster_state   : Normal
------------------------------ nodes information ------------------------------
| node_name | node_ip   | role | port | data_path       |
| node1     | 10.0.0.10 | main | 5432 | /data/gauss/dn1 |
| node2     | 10.0.0.11 | standby | 5432 | /data/gauss/dn2 |
`
	cfg := &config.Config{DBPort: 5432}
	nodes := parseGsOmOutput(output, cfg)

	if len(nodes) != 2 {
		t.Fatalf("parseGsOmOutput() got %d nodes, want 2", len(nodes))
	}
	if nodes[0].Host != "10.0.0.10" {
		t.Errorf("nodes[0].Host = %q, want '10.0.0.10'", nodes[0].Host)
	}
	if nodes[0].Role != "primary" { // "main" -> "primary"
		t.Errorf("nodes[0].Role = %q, want 'primary'", nodes[0].Role)
	}
	if nodes[0].DataDir != "/data/gauss/dn1" {
		t.Errorf("nodes[0].DataDir = %q, want '/data/gauss/dn1'", nodes[0].DataDir)
	}
	if nodes[1].Role != "standby" {
		t.Errorf("nodes[1].Role = %q, want 'standby'", nodes[1].Role)
	}
}

func TestParseGsOmOutput_NoHeader(t *testing.T) {
	output := "some output without the node table header"
	cfg := &config.Config{DBPort: 5432}
	nodes := parseGsOmOutput(output, cfg)
	if len(nodes) != 0 {
		t.Errorf("parseGsOmOutput() without header = %d nodes, want 0", len(nodes))
	}
}

func TestParseGsOmOutput_Empty(t *testing.T) {
	cfg := &config.Config{DBPort: 5432}
	nodes := parseGsOmOutput("", cfg)
	if len(nodes) != 0 {
		t.Errorf("parseGsOmOutput('') = %d nodes, want 0", len(nodes))
	}
}

func TestParseGsOmOutput_SkipsHeaderRow(t *testing.T) {
	// 表头本身匹配 regex，应被过滤掉
	output := `| node_name | node_ip   | role | port | data_path       |
| node_name | node_ip   | role | port | data_path       |
| node1     | 10.0.0.10 | main | 5432 | /data/gauss/dn1 |
`
	cfg := &config.Config{DBPort: 5432}
	nodes := parseGsOmOutput(output, cfg)
	// 只应有 1 个实际节点（表头被过滤）
	for _, n := range nodes {
		if n.Host == "node_ip" {
			t.Error("header row should be filtered out")
		}
	}
}

// ---- OpenGaussAdapter.GetLogPaths ----

func TestOpenGaussAdapter_GetLogPaths(t *testing.T) {
	a := &OpenGaussAdapter{cfg: &config.Config{}}

	// DataDir 为空时返回 nil
	node := NodeInfo{DataDir: ""}
	if paths := a.GetLogPaths(node); paths != nil {
		t.Errorf("GetLogPaths(empty DataDir) = %v, want nil", paths)
	}

	// DataDir 非空时返回 pg_log 和 logs
	node.DataDir = "/data/gauss/dn1"
	paths := a.GetLogPaths(node)
	if len(paths) != 2 {
		t.Fatalf("GetLogPaths() = %d paths, want 2", len(paths))
	}
	foundPgLog, foundLogs := false, false
	for _, p := range paths {
		if p == "/data/gauss/dn1/pg_log" {
			foundPgLog = true
		}
		if p == "/data/gauss/dn1/logs" {
			foundLogs = true
		}
	}
	if !foundPgLog {
		t.Error("GetLogPaths() should include pg_log")
	}
	if !foundLogs {
		t.Error("GetLogPaths() should include logs")
	}
}

// ---- OpenGaussAdapter.Detect ----

func TestOpenGaussAdapter_Detect(t *testing.T) {
	a := &OpenGaussAdapter{cfg: &config.Config{}}
	dbType, err := a.Detect()
	if err != nil {
		t.Errorf("Detect() unexpected error: %v", err)
	}
	if dbType != DBTypeOpenGauss {
		t.Errorf("Detect() = %q, want %q", dbType, DBTypeOpenGauss)
	}
}

// ---- GreenplumAdapter.Detect & GetLogPaths ----

func TestGreenplumAdapter_Detect(t *testing.T) {
	a := NewGreenplumAdapter(&config.Config{})
	dbType, err := a.Detect()
	if err != nil {
		t.Errorf("Detect() unexpected error: %v", err)
	}
	if dbType != DBTypeGreenplum {
		t.Errorf("Detect() = %q, want %q", dbType, DBTypeGreenplum)
	}
}

func TestGreenplumAdapter_GetLogPaths(t *testing.T) {
	a := NewGreenplumAdapter(&config.Config{})

	// DataDir 非空时返回 pg_log
	node := NodeInfo{DataDir: "/data/gp/master"}
	paths := a.GetLogPaths(node)
	if len(paths) != 1 {
		t.Fatalf("GetLogPaths() = %d paths, want 1", len(paths))
	}
	if paths[0] != "/data/gp/master/pg_log" {
		t.Errorf("GetLogPaths() = %q, want '/data/gp/master/pg_log'", paths[0])
	}

	// DataDir 为空时 filepath.Join 返回 "pg_log"（相对路径），
	// CollectLogs 层的 isValidRemotePath 会过滤掉非绝对路径，此处仅验证返回长度
	node.DataDir = ""
	emptyPaths := a.GetLogPaths(node)
	if len(emptyPaths) != 1 {
		t.Errorf("GetLogPaths(empty DataDir) = %d paths, want 1", len(emptyPaths))
	}
}

// ---- PostgresAdapter.Detect & GetLogPaths ----

func TestPostgresAdapter_Detect(t *testing.T) {
	a := NewPostgresAdapter(&config.Config{})
	dbType, err := a.Detect()
	if err != nil {
		t.Errorf("Detect() unexpected error: %v", err)
	}
	if dbType != DBTypePostgres {
		t.Errorf("Detect() = %q, want %q", dbType, DBTypePostgres)
	}
}

func TestPostgresAdapter_GetLogPaths(t *testing.T) {
	a := NewPostgresAdapter(&config.Config{})

	node := NodeInfo{DataDir: ""}
	if paths := a.GetLogPaths(node); paths != nil {
		t.Errorf("GetLogPaths(empty DataDir) = %v, want nil", paths)
	}

	node.DataDir = "/var/lib/postgresql/data"
	paths := a.GetLogPaths(node)
	if len(paths) < 1 {
		t.Fatalf("GetLogPaths() = %d paths, want >= 1", len(paths))
	}
	// 应包含 pg_log 或 log 子目录
	found := false
	for _, p := range paths {
		if p == "/var/lib/postgresql/data/pg_log" || p == "/var/lib/postgresql/data/log" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetLogPaths() = %v, should contain pg_log or log", paths)
	}
}

// ---- DBType constants ----

func TestDBTypeConstants(t *testing.T) {
	if DBTypeGreenplum != "greenplum" {
		t.Errorf("DBTypeGreenplum = %q, want 'greenplum'", DBTypeGreenplum)
	}
	if DBTypePostgres != "postgres" {
		t.Errorf("DBTypePostgres = %q, want 'postgres'", DBTypePostgres)
	}
	if DBTypeOpenGauss != "opengauss" {
		t.Errorf("DBTypeOpenGauss = %q, want 'opengauss'", DBTypeOpenGauss)
	}
}
