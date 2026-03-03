package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"dbpacklogs/internal/collector"
	"dbpacklogs/internal/config"
	"dbpacklogs/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var cfg = &config.Config{}

var hostsRaw string

var rootCmd = &cobra.Command{
	Use:   "dbpacklogs",
	Short: "DBpackLogs - 多数据库日志收集打包工具",
	Long: `DBpackLogs 通过 SSH 远程连接 Greenplum/PostgreSQL/openGauss 节点，
自动探测数据库类型，收集 DB 日志和 OS 诊断信息，按时间范围过滤后打包输出。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.Hosts = config.ParseHosts(hostsRaw)

		utils.InitLogger(cfg.Verbose)
		defer utils.Sync()
		log := utils.GetLogger()

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("参数校验失败: %w", err)
		}

		// 检查输出目录是否存在
		if err := checkOutputDir(cfg.Output); err != nil {
			return fmt.Errorf("输出目录检查失败: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigCh
			log.Warnf("收到信号 %v，正在退出...", sig)
			cancel()
		}()

		orch := collector.NewOrchestrator(cfg)
		return orch.Run(ctx)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}

func checkOutputDir(path string) error {
	if path == "" || path == "." {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 目录不存在，自动创建
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("创建输出目录失败: %w", err)
			}
			utils.GetLogger().Infof("已创建输出目录: %s", path)
			return nil
		}
		return fmt.Errorf("检查输出目录失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("输出路径不是目录: %s", path)
	}
	// 检查目录是否可写
	testFile := path + "/.write_test"
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		return fmt.Errorf("输出目录不可写: %s", path)
	}
	os.Remove(testFile)
	return nil
}

func init() {
	// 按功能分组定义 flag，各组独立注册，用于 help 分类展示
	nodeFlags := pflag.NewFlagSet("node", pflag.ContinueOnError)
	sshFlags := pflag.NewFlagSet("ssh", pflag.ContinueOnError)
	dbFlags := pflag.NewFlagSet("db", pflag.ContinueOnError)
	timeFlags := pflag.NewFlagSet("time", pflag.ContinueOnError)
	outputFlags := pflag.NewFlagSet("output", pflag.ContinueOnError)

	// 节点参数（--hosts / --all-hosts 二选一）
	nodeFlags.StringVar(&hostsRaw, "hosts", "", "指定节点列表，逗号分隔（例：10.0.0.10,10.0.0.11）")
	nodeFlags.BoolVar(&cfg.AllHosts, "all-hosts", false, "自动从 /etc/hosts 读取所有节点（与 --hosts 互斥）")

	// SSH 参数
	sshFlags.StringVar(&cfg.SSHUser, "ssh-user", "", "SSH 用户名（默认为当前系统用户）")
	sshFlags.IntVar(&cfg.SSHPort, "ssh-port", 22, "SSH 端口")
	sshFlags.StringVar(&cfg.SSHPassword, "ssh-password", "", "SSH 密码（已配置免密 SSH 时无需指定）")
	sshFlags.StringVar(&cfg.SSHKey, "ssh-key", "", "SSH 私钥路径（默认自动尝试 ~/.ssh/id_rsa、~/.ssh/id_ed25519）")
	sshFlags.BoolVar(&cfg.InsecureHostKey, "insecure-hostkey", false, "跳过 SSH 主机密钥校验，适用于首次连接未知主机（不安全）")

	// 数据库参数
	dbFlags.StringVar(&cfg.DBUser, "db-user", "", "数据库用户名（默认与 SSH 用户一致）")
	dbFlags.IntVar(&cfg.DBPort, "db-port", 5432, "数据库端口")
	dbFlags.StringVar(&cfg.DBPassword, "db-password", "", "数据库密码（peer/trust 认证时无需指定）")
	dbFlags.StringVar(&cfg.DBName, "db-name", "postgres", "数据库名称")

	// 时间参数（支持格式：2006-01-02 15:04:05 / 2006-01-02T15:04:05 / 2006-01-02 / 20060102）
	timeFlags.StringVar(&cfg.StartTime, "start-time", "", "收集起始时间（不填默认最近 3 天，支持格式：2006-01-02 / 2006-01-02 15:04:05）")
	timeFlags.StringVar(&cfg.EndTime, "end-time", "", "收集结束时间（不填默认当前时间）")

	// 输出参数
	outputFlags.StringVar(&cfg.Output, "output", ".", "输出目录（默认当前目录）")
	outputFlags.StringVar(&cfg.PackType, "pack-type", "zip", "打包格式：zip（默认）或 tar（TAR.GZ）")
	outputFlags.BoolVar(&cfg.Verbose, "verbose", false, "启用调试模式（显示 DEBUG 日志）")

	// 将所有分组注册到命令，使解析生效
	rootCmd.Flags().AddFlagSet(nodeFlags)
	rootCmd.Flags().AddFlagSet(sshFlags)
	rootCmd.Flags().AddFlagSet(dbFlags)
	rootCmd.Flags().AddFlagSet(timeFlags)
	rootCmd.Flags().AddFlagSet(outputFlags)

	// 自定义 help，按分组有序展示
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Printf("\nUsage:\n  %s [flags]\n", cmd.Name())

		printFlagGroup("节点参数（--hosts 与 --all-hosts 二选一）", nodeFlags)
		printFlagGroup("SSH 参数", sshFlags)
		printFlagGroup("数据库参数", dbFlags)
		printFlagGroup("时间参数", timeFlags)
		printFlagGroup("输出参数", outputFlags)
	})
}

// printFlagGroup 打印单个 flag 分组的帮助信息
func printFlagGroup(title string, fs *pflag.FlagSet) {
	fmt.Printf("\n%s:\n", title)
	fmt.Print(fs.FlagUsages())
}
