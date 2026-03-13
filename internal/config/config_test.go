package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseHosts(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"10.0.0.1", []string{"10.0.0.1"}},
		{"10.0.0.1,10.0.0.2", []string{"10.0.0.1", "10.0.0.2"}},
		{"10.0.0.1, 10.0.0.2 , 10.0.0.3", []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}},
		{",,,", nil},
		{"  ,10.0.0.1,  ", []string{"10.0.0.1"}},
	}
	for _, tt := range tests {
		got := ParseHosts(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("ParseHosts(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParseHosts(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"10.0.0.1", true},
		{"192.168.1.100", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"256.0.0.1", false},
		{"10.0.0", false},
		{"10.0.0.1.2", false},
		{"abc.def.ghi.jkl", false},
		{"10.0.0.", false},
		{".10.0.0.1", false},
		{"", false},
		{"10.0.0.1000", false},
	}
	for _, tt := range tests {
		got := IsIP(tt.input)
		if got != tt.want {
			t.Errorf("IsIP(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseEtcHosts(t *testing.T) {
	// 创建临时的 hosts 文件并通过 monkey-patching 测试
	// 由于 ParseEtcHosts 硬编码读取 /etc/hosts，我们通过写临时文件来测试解析逻辑
	// 这里直接测试实际的 /etc/hosts 文件（仅验证不报错）
	ips, err := ParseEtcHosts()
	if err != nil {
		// /etc/hosts 不存在时跳过
		if os.IsNotExist(err) {
			t.Skip("/etc/hosts not found")
		}
		t.Fatalf("ParseEtcHosts() unexpected error: %v", err)
	}
	// 所有返回的 IP 应该是合法 IPv4 且不含过滤掉的地址
	for _, ip := range ips {
		if !IsIP(ip) {
			t.Errorf("ParseEtcHosts() returned invalid IP: %s", ip)
		}
		if ip == "127.0.0.1" || ip == "0.0.0.0" || ip == "255.255.255.255" {
			t.Errorf("ParseEtcHosts() should have filtered %s", ip)
		}
		if strings.Contains(ip, ":") {
			t.Errorf("ParseEtcHosts() should have filtered IPv6: %s", ip)
		}
	}
}

func TestValidate_MutualExclusion(t *testing.T) {
	cfg := &Config{
		AllHosts: true,
		Hosts:    []string{"10.0.0.1"},
		PackType: "zip",
	}
	if err := cfg.Initialize(); err == nil {
		t.Error("Initialize() should return error when both --all-hosts and --hosts specified")
	}
}

func TestValidate_NeitherSpecified(t *testing.T) {
	cfg := &Config{
		PackType: "zip",
	}
	if err := cfg.Initialize(); err == nil {
		t.Error("Initialize() should return error when neither --all-hosts nor --hosts specified")
	}
}

func TestValidate_InvalidPackType(t *testing.T) {
	cfg := &Config{
		Hosts:    []string{"10.0.0.1"},
		PackType: "bz2",
	}
	if err := cfg.Initialize(); err == nil {
		t.Error("Initialize() should return error for invalid pack type")
	}
}

func TestValidate_SSHUserDefault(t *testing.T) {
	cfg := &Config{
		Hosts:    []string{"10.0.0.1"},
		PackType: "zip",
		SSHUser:  "", // 未指定，应自动填充
	}
	if err := cfg.Initialize(); err != nil {
		t.Fatalf("Initialize() unexpected error: %v", err)
	}
	if cfg.SSHUser == "" {
		t.Error("Initialize() should auto-fill SSHUser from current OS user")
	}
}

func TestValidate_DBUserDefaultsToSSHUser(t *testing.T) {
	cfg := &Config{
		Hosts:    []string{"10.0.0.1"},
		PackType: "zip",
		SSHUser:  "testuser",
		DBUser:   "", // 未指定，应与 SSHUser 一致
	}
	if err := cfg.Initialize(); err != nil {
		t.Fatalf("Initialize() unexpected error: %v", err)
	}
	if cfg.DBUser != "testuser" {
		t.Errorf("Initialize() DBUser = %q, want %q", cfg.DBUser, "testuser")
	}
}

func TestValidate_DBHostFromFirstHost(t *testing.T) {
	cfg := &Config{
		Hosts:    []string{"10.0.0.10", "10.0.0.11"},
		PackType: "zip",
		SSHUser:  "user",
	}
	if err := cfg.Initialize(); err != nil {
		t.Fatalf("Initialize() unexpected error: %v", err)
	}
	if cfg.DBHost != "10.0.0.10" {
		t.Errorf("Initialize() DBHost = %q, want %q", cfg.DBHost, "10.0.0.10")
	}
}

func TestValidate_ValidPackTypes(t *testing.T) {
	for _, pt := range []string{"zip", "tar"} {
		cfg := &Config{
			Hosts:    []string{"10.0.0.1"},
			PackType: pt,
			SSHUser:  "user",
		}
		if err := cfg.Initialize(); err != nil {
			t.Errorf("Initialize() with pack-type=%q unexpected error: %v", pt, err)
		}
	}
}
