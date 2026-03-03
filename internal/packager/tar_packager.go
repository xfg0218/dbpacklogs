package packager

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TarPackager 使用 archive/tar + compress/gzip 将目录打包为 .tar.gz
type TarPackager struct{}

// Pack 将 srcDir 下的所有文件递归打包到 destFile（.tar.gz 格式）。
// 使用 fs.WalkDir 替代 filepath.Walk，减少 stat 调用次数，提升大目录性能。
// 同时写入目录头，保证解压后目录结构完整。
func (p *TarPackager) Pack(srcDir string, destFile string) error {
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 检查源目录是否为空
	isEmpty := true
	if err := fs.WalkDir(os.DirFS(srcDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			isEmpty = false
			return fs.SkipAll
		}
		return nil
	}); err != nil {
		return fmt.Errorf("检查源目录失败: %w", err)
	}

	if isEmpty {
		return fmt.Errorf("源目录为空，无文件可打包: %s", srcDir)
	}

	out, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("创建 tar.gz 文件 %s 失败: %w", destFile, err)
	}
	defer out.Close()

	gw, err := gzip.NewWriterLevel(out, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("创建 gzip writer 失败: %w", err)
	}

	tw := tar.NewWriter(gw)

	baseDir := filepath.Dir(srcDir)

	walkErr := fs.WalkDir(os.DirFS(srcDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		absPath := filepath.Join(srcDir, path)
		relPath, err := filepath.Rel(baseDir, absPath)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath
		// 目录名加斜杠，符合 POSIX tar 规范
		if d.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(absPath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})

	// 显式 Close 以捕获 flush/结束标记写入错误，不能依赖 defer
	if err := tw.Close(); err != nil && walkErr == nil {
		return fmt.Errorf("写入 tar 结束标记失败: %w", err)
	}
	if err := gw.Close(); err != nil && walkErr == nil {
		return fmt.Errorf("写入 gzip 结束标记失败: %w", err)
	}
	return walkErr
}
