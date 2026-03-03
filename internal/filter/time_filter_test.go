package filter

import (
	"strings"
	"testing"
	"time"
)

func TestNewTimeFilter_Defaults(t *testing.T) {
	tf, err := NewTimeFilter("", "")
	if err != nil {
		t.Fatalf("NewTimeFilter(\"\", \"\") unexpected error: %v", err)
	}
	now := time.Now()
	// 默认 start 约为 now-72h，end 约为 now
	if tf.End.After(now.Add(time.Second)) {
		t.Errorf("End time %v should be <= now %v", tf.End, now)
	}
	diff := tf.End.Sub(tf.Start)
	if diff < 71*time.Hour || diff > 73*time.Hour {
		t.Errorf("Default range %v should be ~72h", diff)
	}
}

func TestNewTimeFilter_ValidRange(t *testing.T) {
	tf, err := NewTimeFilter("2026-02-20", "2026-02-27")
	if err != nil {
		t.Fatalf("NewTimeFilter() unexpected error: %v", err)
	}
	if tf.Start.Year() != 2026 || tf.Start.Month() != time.February || tf.Start.Day() != 20 {
		t.Errorf("Start = %v, want 2026-02-20", tf.Start)
	}
	if tf.End.Year() != 2026 || tf.End.Month() != time.February || tf.End.Day() != 27 {
		t.Errorf("End = %v, want 2026-02-27", tf.End)
	}
}

func TestNewTimeFilter_EndBeforeStart(t *testing.T) {
	_, err := NewTimeFilter("2026-02-27", "2026-02-20")
	if err == nil {
		t.Error("NewTimeFilter() should return error when end <= start")
	}
}

func TestNewTimeFilter_EqualStartEnd(t *testing.T) {
	_, err := NewTimeFilter("2026-02-20", "2026-02-20")
	if err == nil {
		t.Error("NewTimeFilter() should return error when end == start")
	}
}

func TestNewTimeFilter_InvalidStartTime(t *testing.T) {
	_, err := NewTimeFilter("not-a-date", "2026-02-27")
	if err == nil {
		t.Error("NewTimeFilter() should return error for invalid start time")
	}
}

func TestNewTimeFilter_InvalidEndTime(t *testing.T) {
	_, err := NewTimeFilter("2026-02-20", "not-a-date")
	if err == nil {
		t.Error("NewTimeFilter() should return error for invalid end time")
	}
}

func TestNewTimeFilter_MaxRange90Days(t *testing.T) {
	// 超过 90 天应自动截断
	tf, err := NewTimeFilter("2025-01-01", "2026-12-31")
	if err != nil {
		t.Fatalf("NewTimeFilter() unexpected error: %v", err)
	}
	diff := tf.End.Sub(tf.Start)
	maxAllowed := 90*24*time.Hour + time.Second
	if diff > maxAllowed {
		t.Errorf("Range %v exceeds 90 days limit", diff)
	}
}

func TestNewTimeFilter_FutureEndAdjusted(t *testing.T) {
	// 指定未来时间作为 end，应被调整为 now
	future := time.Now().Add(48 * time.Hour).Format("2006-01-02 15:04:05")
	tf, err := NewTimeFilter("2026-01-01", future)
	if err != nil {
		t.Fatalf("NewTimeFilter() unexpected error: %v", err)
	}
	now := time.Now()
	if tf.End.After(now.Add(2 * time.Second)) {
		t.Errorf("End %v should have been adjusted to now %v", tf.End, now)
	}
}

func TestTimeFilter_FindArgs(t *testing.T) {
	tf := &TimeFilter{
		Start: time.Date(2026, 2, 20, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, 2, 27, 15, 30, 0, 0, time.Local),
	}
	got := tf.FindArgs()
	if !strings.Contains(got, `"2026-02-20 00:00:00"`) {
		t.Errorf("FindArgs() = %q, missing start time", got)
	}
	if !strings.Contains(got, `"2026-02-27 15:30:00"`) {
		t.Errorf("FindArgs() = %q, missing end time", got)
	}
	if !strings.Contains(got, "-newermt") {
		t.Errorf("FindArgs() = %q, missing -newermt", got)
	}
	if !strings.Contains(got, "! -newermt") {
		t.Errorf("FindArgs() = %q, missing ! -newermt", got)
	}
}

func TestTimeFilter_JournalctlArgs(t *testing.T) {
	tf := &TimeFilter{
		Start: time.Date(2026, 2, 20, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, 2, 27, 15, 30, 0, 0, time.Local),
	}
	got := tf.JournalctlArgs()
	if !strings.Contains(got, "--since") {
		t.Errorf("JournalctlArgs() = %q, missing --since", got)
	}
	if !strings.Contains(got, "--until") {
		t.Errorf("JournalctlArgs() = %q, missing --until", got)
	}
	if !strings.Contains(got, "2026-02-20 00:00:00") {
		t.Errorf("JournalctlArgs() = %q, missing start time", got)
	}
	if !strings.Contains(got, "2026-02-27 15:30:00") {
		t.Errorf("JournalctlArgs() = %q, missing end time", got)
	}
}

func TestFilterDmesg_WithTimestamps(t *testing.T) {
	tf := &TimeFilter{
		Start: time.Date(2026, 2, 21, 12, 0, 0, 0, time.Local),
		End:   time.Date(2026, 2, 21, 14, 0, 0, 0, time.Local),
	}

	input := `[Tue Feb 21 11:59:59 2026] before range - should be excluded
[Tue Feb 21 12:00:00 2026] at start - should be included
[Tue Feb 21 13:00:00 2026] in range - should be included
[Tue Feb 21 14:00:00 2026] at end - should be excluded (end is exclusive)
[Tue Feb 21 15:00:00 2026] after range - should be excluded
`
	got := string(tf.FilterDmesg([]byte(input)))

	if strings.Contains(got, "11:59:59") {
		t.Error("FilterDmesg should exclude line before range")
	}
	if !strings.Contains(got, "12:00:00") {
		t.Error("FilterDmesg should include line at start (inclusive)")
	}
	if !strings.Contains(got, "13:00:00") {
		t.Error("FilterDmesg should include line in range")
	}
	if strings.Contains(got, "14:00:00") {
		t.Error("FilterDmesg should exclude line at end (exclusive)")
	}
	if strings.Contains(got, "15:00:00") {
		t.Error("FilterDmesg should exclude line after range")
	}
}

func TestFilterDmesg_UnparsableLines(t *testing.T) {
	tf := &TimeFilter{
		Start: time.Date(2026, 2, 21, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, 2, 22, 0, 0, 0, 0, time.Local),
	}
	input := "# This is a header line without timestamp\nsome random text\n"
	got := string(tf.FilterDmesg([]byte(input)))
	// 无法解析时间戳的行应保留
	if !strings.Contains(got, "header line") {
		t.Error("FilterDmesg should keep lines without parseable timestamp")
	}
	if !strings.Contains(got, "random text") {
		t.Error("FilterDmesg should keep lines without parseable timestamp")
	}
}

func TestFilterDmesg_EmptyInput(t *testing.T) {
	tf := &TimeFilter{
		Start: time.Date(2026, 2, 21, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, 2, 22, 0, 0, 0, 0, time.Local),
	}
	got := tf.FilterDmesg([]byte{})
	if len(got) != 0 {
		t.Errorf("FilterDmesg(empty) = %q, want empty", got)
	}
}

func TestParseDmesgTimestamp_BracketFormat(t *testing.T) {
	tests := []struct {
		line    string
		wantErr bool
		wantHour int
	}{
		{"[Tue Feb 21 12:30:00 2026] some message", false, 12},
		{"[Tue Feb  2 09:00:00 2026] single digit day", false, 9},
		{"[invalid bracket] message", true, 0},
		{"no bracket at all", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got, err := parseDmesgTimestamp(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDmesgTimestamp(%q) expected error, got %v", tt.line, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseDmesgTimestamp(%q) unexpected error: %v", tt.line, err)
				return
			}
			if got.Hour() != tt.wantHour {
				t.Errorf("parseDmesgTimestamp(%q).Hour() = %d, want %d", tt.line, got.Hour(), tt.wantHour)
			}
		})
	}
}

func TestIsMonth(t *testing.T) {
	validMonths := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	for _, m := range validMonths {
		if !isMonth(m) {
			t.Errorf("isMonth(%q) = false, want true", m)
		}
	}
	invalid := []string{"jan", "JAN", "January", "Abc", "", "Ju"}
	for _, m := range invalid {
		if isMonth(m) {
			t.Errorf("isMonth(%q) = true, want false", m)
		}
	}
}
