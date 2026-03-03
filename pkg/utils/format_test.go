package utils

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.input)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{500 * time.Millisecond, "500ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
		{59 * time.Second, "59.0s"},
		{60 * time.Second, "1m0s"},
		{90 * time.Second, "1m30s"},
		{3599 * time.Second, "59m59s"},
		{3600 * time.Second, "1h0m"},
		{3661 * time.Second, "1h1m"},
		{7322 * time.Second, "2h2m"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.input)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseTimeFlexible(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		// 期望解析后的年月日时分秒（以本地时区为基准）
		wantYear  int
		wantMonth time.Month
		wantDay   int
		wantHour  int
		wantMin   int
		wantSec   int
	}{
		{
			input: "2026-02-24 08:00:00", wantErr: false,
			wantYear: 2026, wantMonth: time.February, wantDay: 24,
			wantHour: 8, wantMin: 0, wantSec: 0,
		},
		{
			input: "2026-02-24T08:00:00", wantErr: false,
			wantYear: 2026, wantMonth: time.February, wantDay: 24,
			wantHour: 8, wantMin: 0, wantSec: 0,
		},
		{
			input: "2026-02-24T08:00:00+08:00", wantErr: false,
			wantYear: 2026, wantMonth: time.February, wantDay: 24,
		},
		{
			input: "2026-02-24", wantErr: false,
			wantYear: 2026, wantMonth: time.February, wantDay: 24,
			wantHour: 0, wantMin: 0, wantSec: 0,
		},
		{
			input: "20260224", wantErr: false,
			wantYear: 2026, wantMonth: time.February, wantDay: 24,
			wantHour: 0, wantMin: 0, wantSec: 0,
		},
		{input: "  2026-02-24  ", wantErr: false, wantYear: 2026, wantMonth: time.February, wantDay: 24},
		{input: "not-a-date", wantErr: true},
		{input: "", wantErr: true},
		{input: "2026/02/24", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTimeFlexible(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseTimeFlexible(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseTimeFlexible(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Year() != tt.wantYear || got.Month() != tt.wantMonth || got.Day() != tt.wantDay {
				t.Errorf("ParseTimeFlexible(%q) date = %v-%v-%v, want %v-%v-%v",
					tt.input, got.Year(), got.Month(), got.Day(),
					tt.wantYear, tt.wantMonth, tt.wantDay)
			}
			// 对于带时分秒格式，验证时分秒
			if tt.wantHour != 0 || tt.wantMin != 0 || tt.wantSec != 0 {
				if got.Hour() != tt.wantHour || got.Minute() != tt.wantMin || got.Second() != tt.wantSec {
					t.Errorf("ParseTimeFlexible(%q) time = %02d:%02d:%02d, want %02d:%02d:%02d",
						tt.input, got.Hour(), got.Minute(), got.Second(),
						tt.wantHour, tt.wantMin, tt.wantSec)
				}
			}
		})
	}
}
