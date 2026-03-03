package packager

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ZipPackager 使用 archive/zip 将目录打包为 .zip
type ZipPackager struct{}

// Pack 将 srcDir 下的所有文件递归打包到 destFile（.zip 格式）。
// 使用 fs.WalkDir 替代 filepath.Walk，减少 stat 调用次数，提升大目录性能。
func (p *ZipPackager) Pack(srcDir string, destFile string) error {
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
		return fmt.Errorf("创建 zip 文件 %s 失败: %w", destFile, err)
	}
	defer out.Close()

	w := zip.NewWriter(out)

	baseDir := filepath.Dir(srcDir)

	walkErr := fs.WalkDir(os.DirFS(srcDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// zip 内路径使用正斜杠
		absPath := filepath.Join(srcDir, path)
		relPath, err := filepath.Rel(baseDir, absPath)
		if err != nil {
			return err
		}
		zipPath := filepath.ToSlash(relPath)

		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		fh.Name = zipPath
		fh.Method = zip.Deflate

		writer, err := w.CreateHeader(fh)
		if err != nil {
			return err
		}

		f, err := os.Open(absPath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})

	// 显式 Close 以捕获中央目录写入错误，不能依赖 defer
	if err := w.Close(); err != nil && walkErr == nil {
		return fmt.Errorf("写入 zip 中央目录失败: %w", err)
	}
	return walkErr
}
