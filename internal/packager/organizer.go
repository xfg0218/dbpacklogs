package packager

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbpacklogs/internal/detector"
)

// NodePaths 保存单个节点在工作目录中的路径
type NodePaths struct {
	DBInfo string
	DBLogs string
	OSInfo string
}

// Organizer 负责按规范组织工作目录结构
type Organizer struct{}

// NewOrganizer 创建 Organizer
func NewOrganizer() *Organizer {
	return &Organizer{}
}

// NewWorkDir 在 outputBase 下创建以时间戳命名的工作根目录
// 格式：DBpackLogs_<YYYYMMDD_HHmmss>_<随机6位数字>
func NewWorkDir(outputBase string) (string, error) {
	if outputBase == "" {
		outputBase = "."
	}
	// 添加随机数避免同一秒内多次运行冲突
	randomPart := rand.Intn(1000000)
	dirName := fmt.Sprintf("DBpackLogs_%s_%06d", time.Now().Format("20060102_150405"), randomPart)
	workDir := filepath.Join(outputBase, dirName)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("创建工作目录 %s 失败: %w", workDir, err)
	}
	return workDir, nil
}

// NodeDir 为指定节点创建 db_info / db_logs / os_info 三级目录结构，返回各目录路径
// 结构：<workDir>/<db_type>/<host>/db_info|db_logs|os_info
func (o *Organizer) NodeDir(workDir string, node detector.NodeInfo) (NodePaths, error) {
	base := filepath.Join(workDir, string(node.DBType), node.Host)
	paths := NodePaths{
		DBInfo: filepath.Join(base, "db_info"),
		DBLogs: filepath.Join(base, "db_logs"),
		OSInfo: filepath.Join(base, "os_info"),
	}
	for _, dir := range []string{paths.DBInfo, paths.DBLogs, paths.OSInfo} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return NodePaths{}, fmt.Errorf("创建节点目录 %s 失败: %w", dir, err)
		}
	}
	return paths, nil
}

// Extension 返回打包类型对应的文件扩展名
func Extension(packType string) string {
	if packType == "tar" {
		return ".tar.gz"
	}
	return ".zip"
}

// EnsureUniqueFilePath 检查文件是否存在，如存在则添加数字后缀
func EnsureUniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	var base, ext string
	if strings.HasSuffix(path, ".tar.gz") {
		base = path[:len(path)-7]
		ext = ".tar.gz"
	} else {
		base = path[:len(path)-len(filepath.Ext(path))]
		ext = filepath.Ext(path)
	}

	for i := 1; i <= 999; i++ {
		newPath := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
	return path
}
