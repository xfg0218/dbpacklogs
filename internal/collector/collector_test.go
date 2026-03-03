package collector

import (
	"testing"

	"dbpacklogs/internal/detector"
)

// ---- isValidRemotePath ----

func TestIsValidRemotePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/data/pg_log", true},
		{"/var/lib/postgresql/data/pg_log", true},
		{"/data/gauss/dn1/logs", true},
		{"", false},
		{"relative/path", false},
		{"data/pg_log", false},
		{"/data/path with 'quote", false},
		{"/data/path\"double", false},
		{"/data/path;cmd", false},
		{"/data/path|pipe", false},
		{"/data/path&amp", false},
		{"/data/path$(cmd)", false},
		{"/data/path`backtick`", false},
		{"/data/path\nnewline", false},
	}
	for _, tt := range tests {
		got := isValidRemotePath(tt.path)
		if got != tt.want {
			t.Errorf("isValidRemotePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// ---- safeHostName ----

func TestSafeHostName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"10.0.0.1", "10_0_0_1"},
		{"::1", "__1"},
		{"hostname", "hostname"},
		{"10.0.0.1:5432", "10_0_0_1_5432"},
	}
	for _, tt := range tests {
		got := safeHostName(tt.input)
		if got != tt.want {
			t.Errorf("safeHostName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---- shouldCollectTopology ----

func TestShouldCollectTopology(t *testing.T) {
	tests := []struct {
		node detector.NodeInfo
		want bool
	}{
		{detector.NodeInfo{DBType: detector.DBTypeGreenplum, Role: "coordinator"}, true},
		{detector.NodeInfo{DBType: detector.DBTypeGreenplum, Role: "master"}, true},
		{detector.NodeInfo{DBType: detector.DBTypeGreenplum, Role: "primary"}, false},
		{detector.NodeInfo{DBType: detector.DBTypeGreenplum, Role: "mirror"}, false},
		{detector.NodeInfo{DBType: detector.DBTypePostgres, Role: "primary"}, true},
		{detector.NodeInfo{DBType: detector.DBTypePostgres, Role: "standby"}, false},
		{detector.NodeInfo{DBType: detector.DBTypeOpenGauss, Role: "primary"}, false},
		{detector.NodeInfo{DBType: detector.DBTypeOpenGauss, Role: "standby"}, false},
	}
	for _, tt := range tests {
		got := shouldCollectTopology(tt.node)
		if got != tt.want {
			t.Errorf("shouldCollectTopology({DBType:%q, Role:%q}) = %v, want %v",
				tt.node.DBType, tt.node.Role, got, tt.want)
		}
	}
}

// ---- filterNodesByHosts ----

func TestFilterNodesByHosts_PartialMatch(t *testing.T) {
	nodes := []detector.NodeInfo{
		{Host: "10.0.0.1", Role: "coordinator", Port: 5432, DBType: detector.DBTypeGreenplum},
		{Host: "10.0.0.2", Role: "primary", Port: 5432, DBType: detector.DBTypeGreenplum},
		{Host: "10.0.0.3", Role: "mirror", Port: 5432, DBType: detector.DBTypeGreenplum},
	}
	hosts := []string{"10.0.0.1", "10.0.0.2"}
	result := filterNodesByHosts(nodes, hosts, 5432, detector.DBTypeGreenplum)

	if len(result) != 2 {
		t.Fatalf("filterNodesByHosts() = %d nodes, want 2", len(result))
	}
	for _, n := range result {
		if n.Host != "10.0.0.1" && n.Host != "10.0.0.2" {
			t.Errorf("unexpected host %q in result", n.Host)
		}
	}
}

func TestFilterNodesByHosts_AllMatch(t *testing.T) {
	nodes := []detector.NodeInfo{
		{Host: "10.0.0.1", Role: "primary", DBType: detector.DBTypePostgres},
		{Host: "10.0.0.2", Role: "standby", DBType: detector.DBTypePostgres},
	}
	hosts := []string{"10.0.0.1", "10.0.0.2"}
	result := filterNodesByHosts(nodes, hosts, 5432, detector.DBTypePostgres)

	if len(result) != 2 {
		t.Fatalf("filterNodesByHosts() = %d nodes, want 2", len(result))
	}
}

func TestFilterNodesByHosts_NoMatch_FallbackToSyntheticNodes(t *testing.T) {
	// 当 hosts 中的节点不在已发现节点中时，直接构造单节点
	nodes := []detector.NodeInfo{
		{Host: "10.0.0.1", Role: "primary", DBType: detector.DBTypePostgres},
	}
	hosts := []string{"10.0.0.99"}
	result := filterNodesByHosts(nodes, hosts, 5432, detector.DBTypePostgres)

	if len(result) != 1 {
		t.Fatalf("filterNodesByHosts() = %d nodes, want 1 (fallback)", len(result))
	}
	if result[0].Host != "10.0.0.99" {
		t.Errorf("result[0].Host = %q, want '10.0.0.99'", result[0].Host)
	}
	if result[0].Role != "primary" {
		t.Errorf("result[0].Role = %q, want 'primary' (synthetic default)", result[0].Role)
	}
	if result[0].DBType != detector.DBTypePostgres {
		t.Errorf("result[0].DBType = %q, want 'postgres'", result[0].DBType)
	}
	if result[0].Port != 5432 {
		t.Errorf("result[0].Port = %d, want 5432", result[0].Port)
	}
}

func TestFilterNodesByHosts_EmptyDiscoveredNodes(t *testing.T) {
	var nodes []detector.NodeInfo
	hosts := []string{"10.0.0.1"}
	result := filterNodesByHosts(nodes, hosts, 5432, detector.DBTypeGreenplum)

	if len(result) != 1 {
		t.Fatalf("filterNodesByHosts() with empty discovery = %d nodes, want 1", len(result))
	}
	if result[0].Host != "10.0.0.1" {
		t.Errorf("result[0].Host = %q, want '10.0.0.1'", result[0].Host)
	}
}

func TestFilterNodesByHosts_MultipleHostsNoMatch(t *testing.T) {
	var nodes []detector.NodeInfo
	hosts := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	result := filterNodesByHosts(nodes, hosts, 5432, detector.DBTypeGreenplum)

	if len(result) != 3 {
		t.Fatalf("filterNodesByHosts() = %d nodes, want 3", len(result))
	}
}
