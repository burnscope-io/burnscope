package comparator

import (
	"testing"

	"github.com/burnscope-io/burnscope/internal/protocol"
	"github.com/burnscope-io/burnscope/internal/session"
)

// 辅助函数创建测试记录
func testRecord(direction, name string, data []byte) *session.Record {
	return &session.Record{
		Direction: direction,
		Name:      name,
		RawData:   data,
	}
}

func TestComparatorMatch(t *testing.T) {
	// 创建黄金会话
	golden := session.NewSession("/dev/ttyUSB0", 115200, "ESP-FLASH")
	golden.AddRawData(protocol.TX, []byte{0xC0, 0x00, 0x08, 0xC0})
	golden.AddRawData(protocol.RX, []byte{0xC0, 0x01, 0x08, 0xC0})

	c := NewComparator(golden)

	// 第一条：匹配
	result := c.Compare(testRecord("TX", "RAW", []byte{0xC0, 0x00, 0x08, 0xC0}))
	if result.Result != Match {
		t.Errorf("result.Result = %v, want Match", result.Result)
	}

	// 第二条：匹配
	result = c.Compare(testRecord("RX", "RAW", []byte{0xC0, 0x01, 0x08, 0xC0}))
	if result.Result != Match {
		t.Errorf("result.Result = %v, want Match", result.Result)
	}

	matched, diff, total := c.Stats()
	if matched != 2 {
		t.Errorf("matched = %d, want 2", matched)
	}
	if diff != 0 {
		t.Errorf("diff = %d, want 0", diff)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
}

func TestComparatorDiff(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200, "ESP-FLASH")
	golden.AddRawData(protocol.TX, []byte{0xC0, 0x00, 0x08, 0xC0})

	c := NewComparator(golden)

	// 方向不同
	result := c.Compare(testRecord("RX", "RAW", []byte{0xC0, 0x00, 0x08, 0xC0}))
	if result.Result != Diff {
		t.Errorf("result.Result = %v, want Diff for direction mismatch", result.Result)
	}

	c.Reset()

	// 数据不同
	result = c.Compare(testRecord("TX", "RAW", []byte{0xC0, 0x00, 0x09, 0xC0}))
	if result.Result != Diff {
		t.Errorf("result.Result = %v, want Diff for data mismatch", result.Result)
	}
}

func TestComparatorSkip(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200, "ESP-FLASH")
	golden.AddRawData(protocol.TX, []byte{0x01})

	c := NewComparator(golden)

	// 第一条匹配
	c.Compare(testRecord("TX", "RAW", []byte{0x01}))

	// 第二条：基准已结束
	result := c.Compare(testRecord("TX", "RAW", []byte{0x02}))
	if result.Result != Skip {
		t.Errorf("result.Result = %v, want Skip when golden exhausted", result.Result)
	}
}

func TestComparatorProgress(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200, "ESP-FLASH")
	golden.AddRawData(protocol.TX, []byte{0x01})
	golden.AddRawData(protocol.RX, []byte{0x02})
	golden.AddRawData(protocol.TX, []byte{0x03})

	c := NewComparator(golden)

	current, total := c.Progress()
	if current != 0 || total != 3 {
		t.Errorf("Progress() = (%d, %d), want (0, 3)", current, total)
	}

	c.Compare(testRecord("TX", "RAW", []byte{0x01}))
	current, total = c.Progress()
	if current != 1 || total != 3 {
		t.Errorf("Progress() = (%d, %d), want (1, 3)", current, total)
	}
}

func TestComparatorReset(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200, "ESP-FLASH")
	golden.AddRawData(protocol.TX, []byte{0x01})

	c := NewComparator(golden)
	c.Compare(testRecord("TX", "RAW", []byte{0x01}))

	if !c.IsComplete() {
		t.Error("IsComplete() = false, want true")
	}

	c.Reset()

	if c.IsComplete() {
		t.Error("IsComplete() = true after Reset, want false")
	}

	current, _ := c.Progress()
	if current != 0 {
		t.Errorf("Progress() current = %d after Reset, want 0", current)
	}
}

func TestResultString(t *testing.T) {
	tests := []struct {
		result   Result
		expected string
	}{
		{Match, "✓"},
		{Diff, "✗"},
		{Skip, "-"},
	}

	for _, tt := range tests {
		if got := tt.result.String(); got != tt.expected {
			t.Errorf("Result.String() = %s, want %s", got, tt.expected)
		}
	}
}