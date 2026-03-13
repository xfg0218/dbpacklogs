package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dbpacklogs/internal/filter"
)

func newTestTimeFilter(t *testing.T) *filter.TimeFilter {
	t.Helper()
	tf, err := filter.NewTimeFilter("2026-02-20", "2026-02-27", nil)
	if err != nil {
		t.Fatalf("NewTimeFilter: %v", err)
	}
	return tf
}

func TestNewReporter(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("greenplum", 3, tf)
	if r == nil {
		t.Fatal("NewReporter() returned nil")
	}
	if r.dbType != "greenplum" {
		t.Errorf("dbType = %q, want 'greenplum'", r.dbType)
	}
	if r.totalNodes != 3 {
		t.Errorf("totalNodes = %d, want 3", r.totalNodes)
	}
}

func TestReporter_AddSuccess(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("postgres", 1, tf)
	r.AddSuccess("10.0.0.1", "primary", 2*time.Second)

	if len(r.results) != 1 {
		t.Fatalf("results len = %d, want 1", len(r.results))
	}
	res := r.results[0]
	if res.Host != "10.0.0.1" {
		t.Errorf("Host = %q, want '10.0.0.1'", res.Host)
	}
	if res.Role != "primary" {
		t.Errorf("Role = %q, want 'primary'", res.Role)
	}
	if !res.Success {
		t.Error("Success should be true")
	}
	if res.ElapsedMs != 2000 {
		t.Errorf("ElapsedMs = %d, want 2000", res.ElapsedMs)
	}
}

func TestReporter_AddFailure(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("postgres", 1, tf)
	r.AddFailure("10.0.0.2", "SSH connection refused")

	if len(r.results) != 1 {
		t.Fatalf("results len = %d, want 1", len(r.results))
	}
	res := r.results[0]
	if res.Success {
		t.Error("Success should be false")
	}
	if res.Role != "unknown" {
		t.Errorf("Role = %q, want 'unknown'", res.Role)
	}
	if res.Error != "SSH connection refused" {
		t.Errorf("Error = %q, want 'SSH connection refused'", res.Error)
	}
}

func TestReporter_AddFailureWithRole(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("postgres", 1, tf)
	r.AddFailureWithRole("10.0.0.3", "standby", "timeout")

	if len(r.results) != 1 {
		t.Fatalf("results len = %d, want 1", len(r.results))
	}
	res := r.results[0]
	if res.Role != "standby" {
		t.Errorf("Role = %q, want 'standby'", res.Role)
	}
	if res.Success {
		t.Error("Success should be false")
	}
}

func TestReporter_Generate_TextReport(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("greenplum", 2, tf)
	r.AddSuccess("10.0.0.1", "coordinator", 45*time.Second)
	r.AddFailure("10.0.0.2", "connection timeout")
	r.SetTotalDuration(50 * time.Second)

	workDir := t.TempDir()
	if err := r.Generate(workDir); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workDir, "collection_report.txt"))
	if err != nil {
		t.Fatalf("ReadFile(collection_report.txt) error: %v", err)
	}
	text := string(content)

	checks := []string{
		"DBpackLogs 收集报告",
		"greenplum",
		"10.0.0.1",
		"coordinator",
		"10.0.0.2",
		"connection timeout",
		"成功节点",
		"失败节点",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestReporter_Generate_Metadata(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("postgres", 2, tf)
	r.AddSuccess("10.0.0.1", "primary", 30*time.Second)
	r.AddFailure("10.0.0.2", "error msg")
	r.SetTotalDuration(35 * time.Second)

	workDir := t.TempDir()
	if err := r.Generate(workDir); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "metadata.json"))
	if err != nil {
		t.Fatalf("ReadFile(metadata.json) error: %v", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if meta.DBType != "postgres" {
		t.Errorf("DBType = %q, want 'postgres'", meta.DBType)
	}
	if meta.TotalNodes != 2 {
		t.Errorf("TotalNodes = %d, want 2", meta.TotalNodes)
	}
	if meta.SuccessNodes != 1 {
		t.Errorf("SuccessNodes = %d, want 1", meta.SuccessNodes)
	}
	if meta.FailedNodes != 1 {
		t.Errorf("FailedNodes = %d, want 1", meta.FailedNodes)
	}
	if len(meta.Nodes) != 2 {
		t.Errorf("Nodes len = %d, want 2", len(meta.Nodes))
	}
}

func TestReporter_Generate_NilTimeFilter(t *testing.T) {
	r := NewReporter("postgres", 1, nil)
	workDir := t.TempDir()
	err := r.Generate(workDir)
	if err == nil {
		t.Error("Generate() should error when TimeFilter is nil")
	}
}

func TestReporter_ConcurrentAccess(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("postgres", 10, tf)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(i int) {
			if i%2 == 0 {
				r.AddSuccess("host", "primary", time.Second)
			} else {
				r.AddFailure("host", "err")
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if len(r.results) != 10 {
		t.Errorf("results len = %d, want 10 (concurrent writes)", len(r.results))
	}
}

func TestReporter_NoResults(t *testing.T) {
	tf := newTestTimeFilter(t)
	r := NewReporter("postgres", 0, tf)
	r.SetTotalDuration(0)

	workDir := t.TempDir()
	if err := r.Generate(workDir); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(workDir, "metadata.json"))
	var meta Metadata
	json.Unmarshal(data, &meta)
	if meta.SuccessNodes != 0 || meta.FailedNodes != 0 {
		t.Errorf("Expected 0 nodes, got success=%d failed=%d", meta.SuccessNodes, meta.FailedNodes)
	}
}
