package packager

import (
	"crypto/rand"
	"fmt"
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
// 格式：DBpackLogs_<YYYYMMDD_HHmmss>_<随机 6 位数字>
// 使用 crypto/rand 保证随机性和安全性
func NewWorkDir(outputBase string) (string, error) {
	if outputBase == "" {
		outputBase = "."
	}
	// 使用 crypto/rand 生成安全的随机数，避免同一秒内多次运行冲突
	randomPart := make([]byte, 3)
	if _, err := rand.Read(randomPart); err != nil {
		return "", fmt.Errorf("生成随机数失败：%w", err)
	}
	randomNum := int(randomPart[0])<<16 | int(randomPart[1])<<8 | int(randomPart[2])
	randomNum = randomNum % 1000000 // 限制为 6 位数字

	dirName := fmt.Sprintf("DBpackLogs_%s_%06d", time.Now().Format("20060102_150405"), randomNum)
	workDir := filepath.Join(outputBase, dirName)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("创建工作目录 %s 失败：%w", workDir, err)
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
			return NodePaths{}, fmt.Errorf("创建节点目录 %s 失败：%w", dir, err)
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
