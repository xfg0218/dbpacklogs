package packager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbpacklogs/internal/detector"
)

// setupTestDir 创建带文件的临时目录
func setupTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return dir
}

// ---- ZipPackager ----

func TestZipPackager_BasicPack(t *testing.T) {
	srcDir := setupTestDir(t, map[string]string{
		"file1.txt":         "hello",
		"sub/file2.txt":     "world",
		"sub/deep/file.txt": "deep",
	})
	dest := filepath.Join(t.TempDir(), "out.zip")

	p := &ZipPackager{}
	if err := p.Pack(srcDir, dest); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("zip.OpenReader() error: %v", err)
	}
	defer r.Close()

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}
	for _, want := range []string{"file1.txt", "sub/file2.txt", "sub/deep/file.txt"} {
		found := false
		for name := range names {
			if strings.HasSuffix(name, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("zip missing %q, entries: %v", want, names)
		}
	}
}

func TestZipPackager_EmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "out.zip")

	p := &ZipPackager{}
	err := p.Pack(srcDir, dest)
	if err == nil {
		t.Error("Pack() should error on empty dir")
	}
	if !strings.Contains(err.Error(), "为空") {
		t.Errorf("Pack() error = %q, want '为空'", err.Error())
	}
}

func TestZipPackager_FileContent(t *testing.T) {
	content := "hello zip world"
	srcDir := setupTestDir(t, map[string]string{"test.txt": content})
	dest := filepath.Join(t.TempDir(), "out.zip")

	p := &ZipPackager{}
	if err := p.Pack(srcDir, dest); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("zip.OpenReader() error: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "test.txt") {
			rc, _ := f.Open()
			got, _ := io.ReadAll(rc)
			rc.Close()
			if string(got) != content {
				t.Errorf("content = %q, want %q", got, content)
			}
			return
		}
	}
	t.Error("test.txt not found in zip")
}

// ---- TarPackager ----

func TestTarPackager_BasicPack(t *testing.T) {
	srcDir := setupTestDir(t, map[string]string{
		"file1.txt":     "hello",
		"sub/file2.txt": "world",
	})
	dest := filepath.Join(t.TempDir(), "out.tar.gz")

	p := &TarPackager{}
	if err := p.Pack(srcDir, dest); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	f, _ := os.Open(dest)
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader() error: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	names := make(map[string]bool)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next(): %v", err)
		}
		names[hdr.Name] = true
	}

	for _, want := range []string{"file1.txt", "sub/file2.txt"} {
		found := false
		for name := range names {
			if strings.HasSuffix(name, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tar missing %q, entries: %v", want, names)
		}
	}
}

func TestTarPackager_EmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "out.tar.gz")

	p := &TarPackager{}
	err := p.Pack(srcDir, dest)
	if err == nil {
		t.Error("Pack() should error on empty dir")
	}
}

func TestTarPackager_DirectoryEntriesHaveSlash(t *testing.T) {
	srcDir := setupTestDir(t, map[string]string{
		"subdir/file.txt": "content",
	})
	dest := filepath.Join(t.TempDir(), "out.tar.gz")

	p := &TarPackager{}
	if err := p.Pack(srcDir, dest); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	f, _ := os.Open(dest)
	defer f.Close()
	gr, _ := gzip.NewReader(f)
	defer gr.Close()
	tr := tar.NewReader(gr)

	hasDirEntry := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next(): %v", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			hasDirEntry = true
			if !strings.HasSuffix(hdr.Name, "/") {
				t.Errorf("dir entry %q should end with /", hdr.Name)
			}
		}
	}
	if !hasDirEntry {
		t.Error("tar should include directory entries")
	}
}

// ---- NewPackager factory ----

func TestNewPackager(t *testing.T) {
	if _, ok := NewPackager("tar").(*TarPackager); !ok {
		t.Error("NewPackager('tar') should return *TarPackager")
	}
	if _, ok := NewPackager("zip").(*ZipPackager); !ok {
		t.Error("NewPackager('zip') should return *ZipPackager")
	}
	if _, ok := NewPackager("other").(*ZipPackager); !ok {
		t.Error("NewPackager('other') should default to *ZipPackager")
	}
}

// ---- Extension ----

func TestExtension(t *testing.T) {
	tests := []struct{ input, want string }{
		{"tar", ".tar.gz"},
		{"zip", ".zip"},
		{"", ".zip"},
		{"bz2", ".zip"},
	}
	for _, tt := range tests {
		got := Extension(tt.input)
		if got != tt.want {
			t.Errorf("Extension(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---- EnsureUniqueFilePath ----

func TestEnsureUniqueFilePath_NotExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.zip")
	got := EnsureUniqueFilePath(path)
	if got != path {
		t.Errorf("EnsureUniqueFilePath() = %q, want %q", got, path)
	}
}

func TestEnsureUniqueFilePath_ZipCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.zip")
	os.WriteFile(path, []byte{}, 0644)

	got := EnsureUniqueFilePath(path)
	if got == path {
		t.Error("EnsureUniqueFilePath() should return different path when file exists")
	}
	if !strings.Contains(got, "_1") {
		t.Errorf("EnsureUniqueFilePath() = %q, want '_1' suffix", got)
	}
	if !strings.HasSuffix(got, ".zip") {
		t.Errorf("EnsureUniqueFilePath() = %q, should keep .zip extension", got)
	}
}

func TestEnsureUniqueFilePath_TarGzCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.tar.gz")
	os.WriteFile(path, []byte{}, 0644)

	got := EnsureUniqueFilePath(path)
	if got == path {
		t.Error("EnsureUniqueFilePath() should return different path")
	}
	if !strings.HasSuffix(got, ".tar.gz") {
		t.Errorf("EnsureUniqueFilePath() = %q, should keep .tar.gz extension", got)
	}
}

func TestEnsureUniqueFilePath_MultipleCollisions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.zip")
	os.WriteFile(path, []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "out_1.zip"), []byte{}, 0644)
	os.WriteFile(filepath.Join(dir, "out_2.zip"), []byte{}, 0644)

	got := EnsureUniqueFilePath(path)
	if !strings.Contains(got, "_3") {
		t.Errorf("EnsureUniqueFilePath() = %q, want '_3' suffix", got)
	}
}

// ---- NewWorkDir ----

func TestNewWorkDir_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir, err := NewWorkDir(base)
	if err != nil {
		t.Fatalf("NewWorkDir() error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%s) error: %v", dir, err)
	}
	if !info.IsDir() {
		t.Error("NewWorkDir() should create a directory")
	}
	if !strings.HasPrefix(filepath.Base(dir), "DBpackLogs_") {
		t.Errorf("NewWorkDir() = %q, should start with 'DBpackLogs_'", filepath.Base(dir))
	}
}

func TestNewWorkDir_EmptyBaseUsesCurrentDir(t *testing.T) {
	dir, err := NewWorkDir("")
	if err != nil {
		t.Fatalf("NewWorkDir('') error: %v", err)
	}
	defer os.RemoveAll(dir)
	if !strings.HasPrefix(filepath.Base(dir), "DBpackLogs_") {
		t.Errorf("NewWorkDir('') = %q, should start with 'DBpackLogs_'", filepath.Base(dir))
	}
}

func TestNewWorkDir_Uniqueness(t *testing.T) {
	base := t.TempDir()
	dirs := make(map[string]bool)
	for i := 0; i < 5; i++ {
		dir, err := NewWorkDir(base)
		if err != nil {
			t.Fatalf("NewWorkDir() error: %v", err)
		}
		if dirs[dir] {
			t.Errorf("NewWorkDir() returned duplicate: %s", dir)
		}
		dirs[dir] = true
	}
}

// ---- Organizer.NodeDir ----

func TestOrganizer_NodeDir_CreatesSubDirs(t *testing.T) {
	workDir := t.TempDir()
	org := NewOrganizer()
	node := detector.NodeInfo{
		Host:   "10.0.0.1",
		DBType: detector.DBTypePostgres,
	}

	paths, err := org.NodeDir(workDir, node)
	if err != nil {
		t.Fatalf("NodeDir() error: %v", err)
	}

	for _, dir := range []string{paths.DBInfo, paths.DBLogs, paths.OSInfo} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Stat(%s) error: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestOrganizer_NodeDir_PathStructure(t *testing.T) {
	workDir := t.TempDir()
	org := NewOrganizer()
	node := detector.NodeInfo{
		Host:   "10.0.0.1",
		DBType: detector.DBTypeGreenplum,
	}

	paths, err := org.NodeDir(workDir, node)
	if err != nil {
		t.Fatalf("NodeDir() error: %v", err)
	}

	// 验证路径结构：workDir/greenplum/10.0.0.1/db_info 等
	expectedBase := filepath.Join(workDir, string(detector.DBTypeGreenplum), "10.0.0.1")
	if !strings.HasPrefix(paths.DBInfo, expectedBase) {
		t.Errorf("DBInfo = %q, should be under %q", paths.DBInfo, expectedBase)
	}
	if !strings.HasSuffix(paths.DBInfo, "db_info") {
		t.Errorf("DBInfo = %q, should end with 'db_info'", paths.DBInfo)
	}
	if !strings.HasSuffix(paths.DBLogs, "db_logs") {
		t.Errorf("DBLogs = %q, should end with 'db_logs'", paths.DBLogs)
	}
	if !strings.HasSuffix(paths.OSInfo, "os_info") {
		t.Errorf("OSInfo = %q, should end with 'os_info'", paths.OSInfo)
	}
}
