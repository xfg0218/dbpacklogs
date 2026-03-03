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
	flags := rootCmd.Flags()

	// 节点参数
	// --hosts: 指定节点列表，逗号分隔
	// --all-hosts: 从 /etc/hosts 读取所有节点
	flags.BoolVar(&cfg.AllHosts, "all-hosts", false, "收集所有探测到的节点日志")
	flags.StringVar(&hostsRaw, "hosts", "", "指定节点列表（逗号分隔，例：10.0.0.10,10.0.0.11）")

	// SSH 参数
	// --ssh-user: SSH 用户名（可选，默认为当前 OS 用户）
	// --ssh-key: SSH 私钥路径（可选）
	// --insecure-hostkey: 跳过主机密钥校验（不安全）
	// --ssh-password: SSH 密码
	// --ssh-port: SSH 端口
	flags.IntVar(&cfg.SSHPort, "ssh-port", 22, "SSH 端口")
	flags.StringVar(&cfg.SSHUser, "ssh-user", "", "SSH 用户名（默认为当前系统用户）")
	flags.StringVar(&cfg.SSHKey, "ssh-key", "", "SSH 私钥路径（可选）")
	flags.BoolVar(&cfg.InsecureHostKey, "insecure-hostkey", false, "跳过 SSH 主机密钥校验（不安全）")
	flags.StringVar(&cfg.SSHPassword, "ssh-password", "", "SSH 密码")

	// 数据库参数
	// --db-port: 数据库端口（默认 5432）
	// --db-user: 数据库用户名（默认与 --ssh-user 一致）
	// --db-password: 数据库密码（本地 peer 认证时无需指定）
	// --db-name: 数据库名称（默认 postgres）
	flags.IntVar(&cfg.DBPort, "db-port", 5432, "数据库端口")
	flags.StringVar(&cfg.DBUser, "db-user", "", "数据库用户名（默认与 SSH 用户一致）")
	flags.StringVar(&cfg.DBPassword, "db-password", "", "数据库密码（peer 认证时无需指定）")
	flags.StringVar(&cfg.DBName, "db-name", "postgres", "数据库名称")

	// 时间参数
	// --start-time: 收集起始时间（默认最近3天）
	// --end-time: 收集结束时间（默认当前时间）
	// 支持格式：2006-01-02 15:04:05 / 2006-01-02T15:04:05 / 2006-01-02 / 20060102
	flags.StringVar(&cfg.StartTime, "start-time", "", "日志收集起始时间（不填则默认最近3天，格式：2006-01-02 15:04:05）")
	flags.StringVar(&cfg.EndTime, "end-time", "", "日志收集结束时间（不填则为当前时间）")

	// 输出参数
	// --output: 输出目录（默认当前目录）
	// --pack-type: 打包类型（zip/tar，默认 zip）
	// --verbose: 启用调试日志
	flags.StringVar(&cfg.Output, "output", ".", "收集结果保存路径（默认当前目录）")
	flags.StringVar(&cfg.PackType, "pack-type", "zip", "打包类型：zip（默认）或 tar")
	flags.BoolVar(&cfg.Verbose, "verbose", false, "启用调试模式（显示 DEBUG 日志）")
}
