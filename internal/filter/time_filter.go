package filter

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	"dbpacklogs/pkg/utils"
)

const (
	// defaultLookbackDuration 是未指定起始时间时的默认回溯时长（72 小时 = 3 天）
	defaultLookbackDuration = 72 * time.Hour

	// maxTimeRangeDays 是允许的最大时间跨度天数
	maxTimeRangeDays = 90

	// maxTimeRangeDuration 是允许的最大时间跨度
	maxTimeRangeDuration = maxTimeRangeDays * 24 * time.Hour
)

// TimeFilter 持有已解析的开始/结束时间和时区信息
type TimeFilter struct {
	Start    time.Time
	End      time.Time
	Location *time.Location // 用于解析远端日志的时区
}

// NewTimeFilter 构建时间过滤器。
// 若 startStr/endStr 为空，默认收集最近 3 天。
// location 参数用于解析远端日志时间戳，若为 nil 则使用本地时区。
// 验证顺序：
//  1. 解析时间字符串
//  2. 若 end 在未来，截断为当前时间（并给出警告）
//  3. 验证 end > start（截断后再检查，避免误导性错误）
//  4. 若时间跨度超过 90 天，截断 start
func NewTimeFilter(startStr, endStr string, location *time.Location) (*TimeFilter, error) {
	if location == nil {
		location = time.Local
	}

	now := time.Now().In(location)
	start := now.Add(-defaultLookbackDuration) // 默认：最近 3 天
	end := now

	if startStr != "" {
		t, err := utils.ParseTimeFlexible(startStr)
		if err != nil {
			return nil, fmt.Errorf("解析 --start-time 失败：%w", err)
		}
		// 将用户输入的时间转换到目标时区
		start = t.In(location)
	}
	if endStr != "" {
		t, err := utils.ParseTimeFlexible(endStr)
		if err != nil {
			return nil, fmt.Errorf("解析 --end-time 失败：%w", err)
		}
		// 将用户输入的时间转换到目标时区
		end = t.In(location)
	}

	// 先截断未来时间，再验证先后关系，避免截断后错误信息混乱
	if end.After(now) {
		utils.GetLogger().Warnf("指定的结束时间 %s 在未来，已调整为当前时间", end.Format("2006-01-02 15:04:05"))
		end = now
	}

	// 统一在此处校验 end > start（截断之后）
	if !end.After(start) {
		return nil, fmt.Errorf("--end-time 必须晚于 --start-time（结束时间：%s，开始时间：%s）",
			end.Format("2006-01-02 15:04:05"), start.Format("2006-01-02 15:04:05"))
	}

	// 限制最大时间范围
	if end.Sub(start) > maxTimeRangeDuration {
		utils.GetLogger().Warnf("指定的时间范围超过 %d 天，已限制为 %d 天", maxTimeRangeDays, maxTimeRangeDays)
		start = end.Add(-maxTimeRangeDuration)
	}

	return &TimeFilter{
		Start:    start,
		End:      end,
		Location: location,
	}, nil
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
		t, err := parseDmesgTimestamp(line, tf.Location)
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
func parseDmesgTimestamp(line string, loc *time.Location) (time.Time, error) {
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
		return parseDmesgFormat(tsStr, loc)
	}

	// 尝试提取行首时间 Feb 21 12:00:00 或 Feb  2 12:00:00
	parts := strings.Fields(line)
	if len(parts) >= 3 {
		// 检查第一部分是否是月份
		monthPart := parts[0]
		if isMonth(monthPart) {
			// 尝试解析 "Feb 21 12:00:00" 或 "Feb  2 12:00:00"
			tsStr := parts[0] + " " + parts[1] + " " + parts[2]
			if t, err := parseDmesgFormat(tsStr, loc); err == nil {
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
// 使用传入的 location 参数解析，支持跨时区场景
func parseDmesgFormat(tsStr string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}

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
		if t, err := time.ParseInLocation(layout, tsStr, loc); err == nil {
			// 如果没有年份，使用当前年份
			if t.Year() == 0 {
				t = t.AddDate(time.Now().In(loc).Year(), 0, 0)
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间格式：%s", tsStr)
}
