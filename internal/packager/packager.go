package packager

// Packager 定义打包器接口
type Packager interface {
	// Pack 将 srcDir 目录打包到 destFile
	Pack(srcDir string, destFile string) error
}

// NewPackager 工厂函数，根据类型返回对应打包器
// packType: "zip"（默认）或 "tar"
func NewPackager(packType string) Packager {
	if packType == "tar" {
		return &TarPackager{}
	}
	return &ZipPackager{}
}
