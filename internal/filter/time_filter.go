package filter

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	"dbpacklogs/pkg/utils"
)

// TimeFilter 持有已解析的开始/结束时间
type TimeFilter struct {
	Start time.Time
	End   time.Time
}

// NewTimeFilter 构建时间过滤器。
// 若 startStr/endStr 为空，默认收集最近 3 天。
func NewTimeFilter(startStr, endStr string) (*TimeFilter, error) {
	now := time.Now()
	start := now.Add(-72 * time.Hour) // 默认：最近3天
	end := now

	if startStr != "" {
		t, err := utils.ParseTimeFlexible(startStr)
		if err != nil {
			return nil, fmt.Errorf("解析 --start-time 失败: %w", err)
		}
		start = t
	}
	if endStr != "" {
		t, err := utils.ParseTimeFlexible(endStr)
		if err != nil {
			return nil, fmt.Errorf("解析 --end-time 失败: %w", err)
		}
		end = t
	}
	if !end.After(start) {
		return nil, fmt.Errorf("--end-time 必须晚于 --start-time")
	}
	// 警告：如果结束时间在未来，提示用户
	if end.After(now) {
		utils.GetLogger().Warnf("指定的结束时间 %s 在未来，已调整为当前时间", end.Format("2006-01-02 15:04:05"))
		end = now
		if !end.After(start) {
			return nil, fmt.Errorf("--end-time 必须晚于 --start-time（结束时间已调整为当前时间）")
		}
	}
	// 限制最大时间范围为 90 天
	maxDuration := 90 * 24 * time.Hour
	if end.Sub(start) > maxDuration {
		utils.GetLogger().Warnf("指定的时间范围超过 90 天，已限制为 90 天")
		start = end.Add(-maxDuration)
	}
	return &TimeFilter{Start: start, End: end}, nil
}

// FindArgs 生成 find 命令时间过滤参数
// 示例输出：-newermt "2026-02-21 00:00:00" ! -newermt "2026-02-24 15:30:00"
func (tf *TimeFilter) FindArgs() string {
	return fmt.Sprintf(`-newermt "%s" ! -newermt "%s"`,
		tf.Start.Format("2006-01-02 15:04:05"),
		tf.End.Format("2006-01-02 15:04:05"),
	)
}

// JournalctlArgs 生成 journalctl 时间范围参数
func (tf *TimeFilter) JournalctlArgs() string {
	return fmt.Sprintf(`--since "%s" --until "%s"`,
		tf.Start.Format("2006-01-02 15:04:05"),
		tf.End.Format("2006-01-02 15:04:05"),
	)
}

// FilterDmesg 对 `dmesg -T` 输出按时间过滤。
// dmesg -T 输出格式：[Tue Feb 21 12:00:00 2026] message
func (tf *TimeFilter) FilterDmesg(raw []byte) []byte {
	var result bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		t, err := parseDmesgTimestamp(line)
		if err != nil {
			// 无法解析时间戳的行直接保留（如头部信息）
			result.WriteString(line + "\n")
			continue
		}
		if (t.Equal(tf.Start) || t.After(tf.Start)) && t.Before(tf.End) {
			result.WriteString(line + "\n")
		}
	}
	return result.Bytes()
}

// parseDmesgTimestamp 从 dmesg -T 行首提取时间戳
// 支持多种格式：
// - [Tue Feb 21 12:00:00 2026]
// - [Tue Feb  2 12:00:00 2026]
// - Feb 21 12:00:00 hostname kernel:
// - Feb  2 12:00:00 hostname kernel:
func parseDmesgTimestamp(line string) (time.Time, error) {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return time.Time{}, fmt.Errorf("空行")
	}

	// 尝试提取方括号中的时间 [Tue Feb 21 12:00:00 2026]
	if line[0] == '[' {
		end := strings.Index(line, "]")
		if end < 0 {
			return time.Time{}, fmt.Errorf("缺少右括号")
		}
		tsStr := strings.TrimSpace(line[1:end])
		return parseDmesgFormat(tsStr)
	}

	// 尝试提取行首时间 Feb 21 12:00:00 或 Feb  2 12:00:00
	parts := strings.Fields(line)
	if len(parts) >= 3 {
		// 检查第一部分是否是月份
		monthPart := parts[0]
		if isMonth(monthPart) {
			// 尝试解析 "Feb 21 12:00:00" 或 "Feb  2 12:00:00"
			tsStr := parts[0] + " " + parts[1] + " " + parts[2]
			if t, err := parseDmesgFormat(tsStr); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间戳")
}

// isMonth 判断字符串是否为月份缩写
var monthSet = map[string]bool{
	"Jan": true, "Feb": true, "Mar": true, "Apr": true,
	"May": true, "Jun": true, "Jul": true, "Aug": true,
	"Sep": true, "Oct": true, "Nov": true, "Dec": true,
}

func isMonth(s string) bool {
	_, ok := monthSet[s]
	return ok
}

// parseDmesgFormat 尝试多种格式解析 dmesg 时间戳
func parseDmesgFormat(tsStr string) (time.Time, error) {
	formats := []string{
		"Mon Jan 2 15:04:05 2006",
		"Mon Jan  2 15:04:05 2006",
		"Mon Jan  2 15:04:05",
		"Jan 2 15:04:05 2006",
		"Jan  2 15:04:05 2006",
		"Jan 2 15:04:05",
		"Jan  2 15:04:05",
	}

	for _, layout := range formats {
		if t, err := time.ParseInLocation(layout, tsStr, time.Local); err == nil {
			// 如果没有年份，使用当前年份
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间格式: %s", tsStr)
}
