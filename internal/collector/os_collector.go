package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"dbpacklogs/internal/filter"
	intssh "dbpacklogs/internal/ssh"
	"dbpacklogs/pkg/utils"
)

// OSCollector 负责收集节点的 OS 诊断信息
type OSCollector struct{}

// NewOSCollector 创建 OSCollector
func NewOSCollector() *OSCollector {
	return &OSCollector{}
}

// osTask 定义单项 OS 收集任务
type osTask struct {
	name string // 任务名称（用于日志）
	cmd  string // 远端执行的 shell 命令
	file string // 保存到本地的文件名
}

// CollectAll 并发收集 9 类 OS 信息，保存到 osInfoDir 目录。
// 单项失败不影响其他项，所有警告汇总后以日志输出。
func (c *OSCollector) CollectAll(sshClient *intssh.SSHClient, osInfoDir string, tf *filter.TimeFilter) error {
	if err := os.MkdirAll(osInfoDir, 0755); err != nil {
		return fmt.Errorf("创建 os_info 目录失败: %w", err)
	}

	// 预生成 journalctl 参数，避免在 goroutine 中重复处理
	journalArgs := ""
	if tf != nil {
		journalArgs = tf.JournalctlArgs()
	}

	tasks := []osTask{
		{
			name: "cpu",
			cmd:  "lscpu 2>/dev/null || cat /proc/cpuinfo",
			file: "cpu.txt",
		},
		{
			name: "disk",
			cmd:  "df -h; echo '--- lsblk ---'; lsblk 2>/dev/null",
			file: "disk.txt",
		},
		{
			name: "memory",
			cmd:  "free -h; echo '--- /proc/meminfo ---'; cat /proc/meminfo",
			file: "memory.txt",
		},
		{
			name: "os_info",
			cmd:  "uname -a; echo '--- /etc/os-release ---'; cat /etc/os-release 2>/dev/null || cat /etc/redhat-release 2>/dev/null",
			file: "os_info.txt",
		},
		{
			name: "hosts",
			cmd:  "cat /etc/hosts",
			file: "hosts.txt",
		},
		{
			name: "dmesg",
			// dmesg 不支持时间参数，先获取完整输出在本地过滤
			cmd:  "dmesg -T 2>/dev/null || dmesg 2>/dev/null",
			file: "dmesg.txt",
		},
		{
			name: "journalctl",
			// command -v 检测可用性，兼容无 systemd 的环境
			cmd:  fmt.Sprintf("command -v journalctl >/dev/null 2>&1 && journalctl %s --no-pager 2>/dev/null || echo '[journalctl 不可用]'", journalArgs),
			file: "journalctl.txt",
		},
		{
			name: "raid",
			cmd:  "cat /proc/mdstat 2>/dev/null; command -v mdadm >/dev/null 2>&1 && mdadm --detail --scan 2>/dev/null || echo '[mdadm 不可用]'",
			file: "raid.txt",
		},
		{
			name: "network",
			cmd:  "ip addr 2>/dev/null || ifconfig 2>/dev/null; echo '--- routes ---'; ip route 2>/dev/null || netstat -rn 2>/dev/null",
			file: "network.txt",
		},
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, t := range tasks {
		wg.Add(1)
		go func(task osTask) {
			defer wg.Done()
			localPath := filepath.Join(osInfoDir, task.file)
			if err := c.collectCommand(sshClient, task.cmd, localPath, task.name, tf); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()

	if len(errs) > 0 {
		log := utils.GetLogger()
		for _, e := range errs {
			log.Warnf("OS 信息收集警告: %v", e)
		}
	}
	return nil
}

// collectCommand 在远端执行命令，将 stdout 保存到本地文件。
// 对 dmesg 特殊处理：在本地执行时间范围过滤。
// 通过追加 `|| true` 保证远端命令非零退出不触发 SSH 错误。
func (c *OSCollector) collectCommand(
	sshClient *intssh.SSHClient,
	cmd string,
	localFile string,
	taskName string,
	tf *filter.TimeFilter,
) error {
	cmd = fmt.Sprintf("%s || true", cmd)
	out, err := sshClient.Execute(cmd)
	if err != nil {
		return fmt.Errorf("[%s] 执行失败: %w", taskName, err)
	}

	// dmesg 特殊处理：本地过滤时间范围
	if taskName == "dmesg" && len(out) > 0 && tf != nil {
		out = tf.FilterDmesg(out)
	}

	if err := os.WriteFile(localFile, out, 0644); err != nil {
		return fmt.Errorf("[%s] 写入本地文件 %s 失败: %w", taskName, localFile, err)
	}
	utils.GetLogger().Debugf("[%s] 收集完成 -> %s (%s)", taskName, localFile, utils.FormatBytes(int64(len(out))))
	return nil
}
