package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/sftp"
)

const (
	sftpBufSize = 4 * 1024 * 1024
)

// RemoteCompress 在远端执行 tar 将指定文件列表流式压缩写回 writer。
// 通过 stdin pipe 传递文件列表，避免命令行过长；
// session.Stdout 直接接到 writer 实现零拷贝流式传输。
func RemoteCompress(client *SSHClient, remotePaths []string, writer io.Writer) error {
	if len(remotePaths) == 0 {
		return nil
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取 stdin pipe 失败: %w", err)
	}

	session.Stdout = writer

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdinPipe.Close()
		for _, p := range remotePaths {
			fmt.Fprintln(stdinPipe, p)
		}
	}()

	runErr := session.Run("tar czf - --files-from=-")
	wg.Wait() // 确保 goroutine 退出，不论 Run 是否成功
	if runErr != nil {
		if stderrBuf.Len() > 0 {
			return fmt.Errorf("远端 tar 执行失败: %w; stderr: %s", runErr, stderrBuf.String())
		}
		return fmt.Errorf("远端 tar 执行失败: %w", runErr)
	}
	return nil
}

// FindRemoteFiles 在远端执行 find 命令，返回匹配的文件路径列表。
// findArgs 为时间过滤参数，例如 `-newermt "2026-02-21" ! -newermt "2026-02-24"`。
// 使用 stdin 传递目录路径，避免命令注入风险。
func FindRemoteFiles(client *SSHClient, dir string, findArgs string) ([]string, error) {
	// 验证路径安全性
	if !isValidPath(dir) {
		return nil, fmt.Errorf("路径 %s 包含不安全字符", dir)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建 SSH session 失败：%w", err)
	}
	defer session.Close()

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("获取 stdin pipe 失败：%w", err)
	}

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	// 通过 stdin 传递目录路径，避免命令注入
	// find 命令从 stdin 读取目录列表，每个目录一行
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdinPipe.Close()
		// 将目录路径写入 stdin（find 会从 stdin 读取搜索路径）
		fmt.Fprintln(stdinPipe, dir)
	}()

	// 使用 --files-from=- 从 stdin 读取搜索路径
	// 注意：find 的 --files-from 是 GNU 扩展，大多数 Linux 系统支持
	cmd := fmt.Sprintf("find --files-from=- -type f %s 2>/dev/null", findArgs)
	if err := session.Run(cmd); err != nil {
		wg.Wait()
		return nil, fmt.Errorf("远端 find 执行失败：%w", err)
	}
	wg.Wait()

	var files []string
	scanner := bufio.NewScanner(&stdoutBuf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// isValidPath 验证路径安全性
// - 必须是绝对路径
// - 不能包含 .. 路径分量（目录遍历）
// - 不能包含 NUL 字节或其他控制字符
// - 不能包含 shell 危险字符
func isValidPath(path string) bool {
	if path == "" || path[0] != '/' {
		return false
	}
	// 规范化路径后检查（防止 /foo/../etc/passwd 等绕过）
	cleaned := filepath.Clean(path)
	// 规范化后仍需以 / 开头
	if cleaned == "" || cleaned[0] != '/' {
		return false
	}
	// 禁止包含 .. 分量（filepath.Clean 会折叠，但明确拒绝原始输入中的 ..）
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return false
		}
	}
	// 禁止 NUL 字节及其他控制字符（ASCII 0x00–0x1F）
	for _, r := range path {
		if r < 0x20 {
			return false
		}
	}
	// 禁止 shell 危险字符
	dangerousChars := []string{"'", "\"", "`", ";", "|", "&", "$", "(", ")", "{", "}", "[", "]", "<", ">"}
	for _, c := range dangerousChars {
		if strings.Contains(path, c) {
			return false
		}
	}
	return true
}

// DownloadFile 通过 SFTP 将远端单个文件下载到本地路径（适合配置文件等小文件）。
// 使用 bufio.Writer 缓冲写入，减少系统调用次数。
func DownloadFile(client *SSHClient, remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(client.UnderlyingClient())
	if err != nil {
		return fmt.Errorf("SFTP 初始化失败: %w", err)
	}
	defer sftpClient.Close()

	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("打开远程文件 %s 失败: %w", remotePath, err)
	}
	defer remoteFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件 %s 失败: %w", localPath, err)
	}
	defer localFile.Close()

	// 使用带缓冲的 writer 提升写性能
	bw := bufio.NewWriterSize(localFile, sftpBufSize)
	if _, err = io.Copy(bw, remoteFile); err != nil {
		return fmt.Errorf("下载文件 %s 失败: %w", remotePath, err)
	}
	return bw.Flush()
}
