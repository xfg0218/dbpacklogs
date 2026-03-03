package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbpacklogs/internal/config"
	"dbpacklogs/internal/detector"
	"dbpacklogs/internal/filter"
	"dbpacklogs/internal/packager"
	"dbpacklogs/internal/report"
	intssh "dbpacklogs/internal/ssh"
	"dbpacklogs/pkg/utils"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Orchestrator 编排整个日志收集流程
type Orchestrator struct {
	cfg *config.Config
}

// NewOrchestrator 创建编排器
func NewOrchestrator(cfg *config.Config) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

// Run 执行完整的收集流程：探测 → 节点发现 → 并发收集 → 报告 → 打包
func (o *Orchestrator) Run(ctx context.Context) error {
	log := utils.GetLogger()
	startTime := time.Now()

	// 0. 初始化时间过滤器
	tf, err := filter.NewTimeFilter(o.cfg.StartTime, o.cfg.EndTime)
	if err != nil {
		return fmt.Errorf("初始化时间过滤器失败: %w", err)
	}
	log.Infof("时间范围: %s ~ %s",
		tf.Start.Format("2006-01-02 15:04:05"),
		tf.End.Format("2006-01-02 15:04:05"))

	// 1. 创建本地工作目录
	workDir, err := packager.NewWorkDir(o.cfg.Output)
	if err != nil {
		return fmt.Errorf("创建工作目录失败: %w", err)
	}
	log.Infof("工作目录: %s", workDir)
	// packSuccess 标记打包是否成功，仅打包成功后才清理临时目录
	// 若打包失败，保留临时目录，便于用户手动检索已收集的数据
	packSuccess := false
	defer func() {
		if !packSuccess {
			log.Warnf("打包未完成，临时工作目录已保留: %s", workDir)
			return
		}
		if err := os.RemoveAll(workDir); err != nil {
			log.Warnf("清理临时工作目录 %s 失败: %v", workDir, err)
		} else {
			log.Debugf("已清理临时工作目录: %s", workDir)
		}
	}()

	// 2. 初始化 SSH 连接池
	sshPool := intssh.NewPool(o.cfg)
	defer sshPool.CloseAll()

	// --all-hosts 模式下 DBHost 可能为空，需要先有一个可用 SSH 节点
	// 此时无法通过 DB 连接探测类型，先尝试 SSH 到第一个 --hosts 节点或跳板机节点
	// factory.go 内部会处理 DBHost 为空时的 openGauss SSH 回退逻辑

	// 3. 探测数据库类型 & 发现节点
	adapter, err := detector.NewAdapter(o.cfg, sshPool)
	if err != nil {
		return fmt.Errorf("数据库探测失败: %w", err)
	}

	dbType, err := adapter.Detect()
	if err != nil {
		return fmt.Errorf("获取数据库类型失败: %w", err)
	}
	log.Infof("数据库类型: %s", dbType)

	nodes, err := adapter.DiscoverNodes()
	if err != nil {
		return fmt.Errorf("节点发现失败: %w", err)
	}
	log.Infof("发现 %d 个节点", len(nodes))

	// 如果用户通过 --hosts 指定了节点子集，进行过滤
	if !o.cfg.AllHosts && len(o.cfg.Hosts) > 0 {
		nodes = filterNodesByHosts(nodes, o.cfg.Hosts, o.cfg.DBPort, dbType)
		log.Infof("按 --hosts 过滤后剩余 %d 个节点", len(nodes))
	}

	// 4. 并发收集（errgroup 错误隔离）
	org := packager.NewOrganizer()
	dbCollector := NewDBCollector(o.cfg, adapter)
	osCollector := NewOSCollector()

	eg, egCtx := errgroup.WithContext(ctx)

	// 收集报告数据
	rep := report.NewReporter(string(dbType), len(nodes), tf)

	// 限制并发数（默认 10），避免资源耗尽
	maxConcurrency := int64(10)
	if int64(len(nodes)) < maxConcurrency {
		maxConcurrency = int64(len(nodes))
	}
	sem := semaphore.NewWeighted(maxConcurrency)

	for _, node := range nodes {
		node := node // 捕获循环变量
		eg.Go(func() error {
			// 获取信号量，控制并发数
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			// 检查上下文是否被取消
			select {
			case <-egCtx.Done():
				return egCtx.Err()
			default:
			}

			nodeStart := time.Now()
			log.Infof("[%s] 开始收集（角色: %s）", node.Host, node.Role)

			// 获取 SSH 连接
			sshClient, err := sshPool.Get(node.Host, o.cfg.SSHPort)
			if err != nil {
				rep.AddFailure(node.Host, fmt.Sprintf("SSH 连接失败: %v", err))
				log.Errorf("[%s] SSH 连接失败: %v", node.Host, err)
				return nil // 不传播错误，单节点失败不中断其他节点
			}

			// 创建节点目录结构
			paths, err := org.NodeDir(workDir, node)
			if err != nil {
				rep.AddFailure(node.Host, fmt.Sprintf("创建节点目录失败: %v", err))
				log.Errorf("[%s] 创建节点目录失败: %v", node.Host, err)
				return nil
			}

			// 统一补齐 DataDir（仅对 standby 等 DataDir 为空的节点执行一次 psql 查询）
			dbCollector.EnsureDataDir(sshClient, &node)

			// 4a. 收集 DB 配置信息
			var collectionErrors []string
			if err := dbCollector.CollectInfo(sshClient, node, paths.DBInfo); err != nil {
				errMsg := fmt.Sprintf("DB 信息收集失败: %v", err)
				log.Warnf("[%s] %s", node.Host, errMsg)
				collectionErrors = append(collectionErrors, errMsg)
			}

			// 4b. 收集 DB 日志
			if err := dbCollector.CollectLogs(sshClient, node, paths.DBLogs, tf); err != nil {
				errMsg := fmt.Sprintf("DB 日志收集失败: %v", err)
				log.Warnf("[%s] %s", node.Host, errMsg)
				collectionErrors = append(collectionErrors, errMsg)
			}

			// 4c. 收集 OS 信息
			if err := osCollector.CollectAll(sshClient, paths.OSInfo, tf); err != nil {
				errMsg := fmt.Sprintf("OS 信息收集失败: %v", err)
				log.Warnf("[%s] %s", node.Host, errMsg)
				collectionErrors = append(collectionErrors, errMsg)
			}

			elapsed := time.Since(nodeStart)
			// 如果有收集错误，记录到报告中
			if len(collectionErrors) > 0 {
				rep.AddFailureWithRole(node.Host, node.Role, strings.Join(collectionErrors, "; "))
			} else {
				rep.AddSuccess(node.Host, node.Role, elapsed)
			}
			log.Infof("[%s] 收集完成，耗时 %s", node.Host, utils.FormatDuration(elapsed))
			return nil
		})
	}

	if err := eg.Wait(); err != nil && err != context.Canceled {
		return fmt.Errorf("并发收集出错: %w", err)
	}

	// 5. 生成报告
	rep.SetTotalDuration(time.Since(startTime))
	if err := rep.Generate(workDir); err != nil {
		log.Warnf("生成报告失败: %v", err)
	}

	// 6. 打包
	packType := o.cfg.PackType
	packer := packager.NewPackager(packType)
	outputFile := filepath.Join(o.cfg.Output, filepath.Base(workDir)+packager.Extension(packType))

	// 检查输出文件是否存在，避免覆盖
	outputFile = packager.EnsureUniqueFilePath(outputFile)
	log.Infof("开始打包 -> %s", outputFile)

	if err := packer.Pack(workDir, outputFile); err != nil {
		return fmt.Errorf("打包失败: %w", err)
	}
	packSuccess = true

	log.Infof("完成！输出文件: %s，总耗时: %s", outputFile, utils.FormatDuration(time.Since(startTime)))
	return nil
}

// filterNodesByHosts 按用户指定的 hosts 列表过滤节点
func filterNodesByHosts(nodes []detector.NodeInfo, hosts []string, dbPort int, dbType detector.DBType) []detector.NodeInfo {
	hostSet := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		hostSet[h] = struct{}{}
	}
	var filtered []detector.NodeInfo
	for _, n := range nodes {
		if _, ok := hostSet[n.Host]; ok {
			filtered = append(filtered, n)
		}
	}
	// 若过滤后为空（hosts 中有不属于集群的节点），直接添加为当前 DBType 单节点
	if len(filtered) == 0 {
		for _, h := range hosts {
			filtered = append(filtered, detector.NodeInfo{
				Host:   h,
				Port:   dbPort,
				Role:   "primary",
				DBType: dbType,
			})
		}
	}
	return filtered
}
