package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dbpacklogs/internal/filter"
	"dbpacklogs/pkg/utils"
)

// NodeResult 记录单个节点的收集结果
type NodeResult struct {
	Host      string `json:"host"`
	Role      string `json:"role"`
	Success   bool   `json:"success"`
	ElapsedMs int64  `json:"elapsed_ms"` // 使用毫秒整数，避免 JSON 序列化负值问题
	Error     string `json:"error,omitempty"`
}

// Reporter 收集过程中的统计数据，最终生成报告文件
type Reporter struct {
	mu            sync.Mutex
	dbType        string
	totalNodes    int
	tf            *filter.TimeFilter
	results       []NodeResult
	totalDuration time.Duration
	generatedAt   time.Time
}

// Metadata 写入 metadata.json 的结构
type Metadata struct {
	DBType        string       `json:"db_type"`
	TotalNodes    int          `json:"total_nodes"`
	SuccessNodes  int          `json:"success_nodes"`
	FailedNodes   int          `json:"failed_nodes"`
	StartTime     string       `json:"start_time"`
	EndTime       string       `json:"end_time"`
	GeneratedAt   string       `json:"generated_at"`
	TotalDuration string       `json:"total_duration"`
	Nodes         []NodeResult `json:"nodes"`
}

// NewReporter 创建 Reporter
func NewReporter(dbType string, totalNodes int, tf *filter.TimeFilter) *Reporter {
	return &Reporter{
		dbType:      dbType,
		totalNodes:  totalNodes,
		tf:          tf,
		generatedAt: time.Now(),
	}
}

// AddSuccess 记录节点成功结果
func (r *Reporter) AddSuccess(host, role string, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, NodeResult{
		Host:      host,
		Role:      role,
		Success:   true,
		ElapsedMs: elapsed.Milliseconds(),
	})
}

// AddFailure 记录节点失败结果
func (r *Reporter) AddFailure(host, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, NodeResult{
		Host:    host,
		Role:    "unknown",
		Success: false,
		Error:   errMsg,
	})
}

// AddFailureWithRole 记录节点失败结果（带角色信息）
func (r *Reporter) AddFailureWithRole(host, role, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, NodeResult{
		Host:    host,
		Role:    role,
		Success: false,
		Error:   errMsg,
	})
}

// SetTotalDuration 设置总耗时
func (r *Reporter) SetTotalDuration(d time.Duration) {
	r.totalDuration = d
}

// Generate 生成 collection_report.txt 和 metadata.json
func (r *Reporter) Generate(workDir string) error {
	if err := r.writeTextReport(workDir); err != nil {
		return err
	}
	return r.writeMetadata(workDir)
}

func (r *Reporter) writeTextReport(workDir string) error {
	// 检查 tf 是否为空
	if r.tf == nil {
		return fmt.Errorf("TimeFilter 未初始化")
	}

	var successNodes, failedNodes []NodeResult
	for _, nr := range r.results {
		if nr.Success {
			successNodes = append(successNodes, nr)
		} else {
			failedNodes = append(failedNodes, nr)
		}
	}

	outPath := filepath.Join(workDir, "collection_report.txt")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("创建报告文件失败: %w", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "=== DBpackLogs 收集报告 ===")
	fmt.Fprintf(f, "生成时间  : %s\n", r.generatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "数据库类型: %s\n", r.dbType)
	fmt.Fprintf(f, "时间范围  : %s ~ %s\n",
		r.tf.Start.Format("2006-01-02 15:04:05"),
		r.tf.End.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "总节点数  : %d\n", r.totalNodes)
	fmt.Fprintf(f, "成功节点  : %d\n", len(successNodes))
	fmt.Fprintf(f, "失败节点  : %d\n", len(failedNodes))
	fmt.Fprintf(f, "总耗时    : %s\n", utils.FormatDuration(r.totalDuration))
	fmt.Fprintln(f)

	if len(successNodes) > 0 {
		fmt.Fprintln(f, "--- 成功节点 ---")
		for _, n := range successNodes {
			fmt.Fprintf(f, "  [OK] %-20s role=%-12s elapsed=%s\n",
				n.Host, n.Role, utils.FormatDuration(time.Duration(n.ElapsedMs)*time.Millisecond))
		}
		fmt.Fprintln(f)
	}

	if len(failedNodes) > 0 {
		fmt.Fprintln(f, "--- 失败节点 ---")
		for _, n := range failedNodes {
			fmt.Fprintf(f, "  [FAIL] %-20s error=%s\n", n.Host, n.Error)
		}
		fmt.Fprintln(f)
	}

	return nil
}

func (r *Reporter) writeMetadata(workDir string) error {
	// 检查 tf 是否为空
	if r.tf == nil {
		return fmt.Errorf("TimeFilter 未初始化")
	}

	successCount := 0
	for _, nr := range r.results {
		if nr.Success {
			successCount++
		}
	}

	meta := Metadata{
		DBType:        r.dbType,
		TotalNodes:    r.totalNodes,
		SuccessNodes:  successCount,
		FailedNodes:   len(r.results) - successCount,
		StartTime:     r.tf.Start.Format("2006-01-02 15:04:05"),
		EndTime:       r.tf.End.Format("2006-01-02 15:04:05"),
		GeneratedAt:   r.generatedAt.Format("2006-01-02 15:04:05"),
		TotalDuration: utils.FormatDuration(r.totalDuration),
		Nodes:         r.results,
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 metadata 失败: %w", err)
	}

	outPath := filepath.Join(workDir, "metadata.json")
	return os.WriteFile(outPath, data, 0644)
}
