package monitor

import (
	"testing"
)

func TestParseMemInfo_Modern(t *testing.T) {
	content := `MemTotal:        8033660 kB
MemFree:         4343164 kB
MemAvailable:    6035824 kB
Buffers:          255396 kB
Cached:          1813476 kB`

	metrics := &HostMetrics{}
	ParseMemInfo(content, metrics)

	if metrics.MemTotal != 8033660*1024 {
		t.Errorf("expected %d, got %d", 8033660*1024, metrics.MemTotal)
	}
	expectedUsed := uint64(8033660-6035824) * 1024
	if metrics.MemUsed != expectedUsed {
		t.Errorf("expected %d, got %d", expectedUsed, metrics.MemUsed)
	}
}

func TestParseMemInfo_Fallback(t *testing.T) {
	content := `MemTotal:        2048000 kB
MemFree:          500000 kB
Buffers:          100000 kB
Cached:           200000 kB`

	metrics := &HostMetrics{}
	ParseMemInfo(content, metrics)

	// Available = Free + Buffers + Cached = 500k + 100k + 200k = 800000 kB
	// Used = Total - Available = 2048000 - 800000 = 1248000 kB
	expectedUsed := uint64(1248000) * 1024

	if metrics.MemUsed != expectedUsed {
		t.Errorf("expected %d, got %d", expectedUsed, metrics.MemUsed)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "N/A"},
		{524288000, "500 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
	}

	for _, tc := range tests {
		actual := FormatBytes(tc.bytes)
		if actual != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, actual)
		}
	}
}
