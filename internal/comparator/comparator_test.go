package comparator

import (
	"testing"

	"github.com/burnscope-io/burnscope/internal/session"
)

func TestComparatorMatch(t *testing.T) {
	// 创建黄金会话: TX, RX, TX, RX
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0xC0, 0x00, 0x08, 0xC0}) // TX #1
	golden.Add(session.RX, []byte{0xC0, 0x01, 0x08, 0xC0}) // RX #1
	golden.Add(session.TX, []byte{0xC0, 0x0B, 0x00, 0xC0}) // TX #2
	golden.Add(session.RX, []byte{0xC0, 0x0C, 0x00, 0xC0}) // RX #2

	c := NewComparator(golden)

	// 第一条 TX：匹配
	result := c.Compare(&session.Record{Direction: session.TX, Data: []byte{0xC0, 0x00, 0x08, 0xC0}})
	if result.Result != Match {
		t.Errorf("result.Result = %v, want Match", result.Result)
	}
	if result.ExpectedTX == nil {
		t.Fatal("ExpectedTX is nil")
	}
	if result.ExpectedRX == nil {
		t.Fatal("ExpectedRX is nil") // 应该有对应的 RX
	}

	// 第二条 TX：匹配
	result = c.Compare(&session.Record{Direction: session.TX, Data: []byte{0xC0, 0x0B, 0x00, 0xC0}})
	if result.Result != Match {
		t.Errorf("result.Result = %v, want Match", result.Result)
	}

	matched, diff, _ := c.Stats()
	if matched != 2 {
		t.Errorf("matched = %d, want 2", matched)
	}
	if diff != 0 {
		t.Errorf("diff = %d, want 0", diff)
	}
}

func TestComparatorDiff(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0xC0, 0x00, 0x08, 0xC0})
	golden.Add(session.RX, []byte{0xC0, 0x01, 0x08, 0xC0})

	c := NewComparator(golden)

	// 数据不同
	result := c.Compare(&session.Record{Direction: session.TX, Data: []byte{0xC0, 0x00, 0x09, 0xC0}})
	if result.Result != Diff {
		t.Errorf("result.Result = %v, want Diff for data mismatch", result.Result)
	}

	matched, diff, _ := c.Stats()
	if matched != 0 {
		t.Errorf("matched = %d, want 0", matched)
	}
	if diff != 1 {
		t.Errorf("diff = %d, want 1", diff)
	}
}

func TestComparatorSkip(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0x01})

	c := NewComparator(golden)

	// 第一条匹配
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})

	// 第二条：基准已结束
	result := c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x02}})
	if result.Result != Skip {
		t.Errorf("result.Result = %v, want Skip when golden exhausted", result.Result)
	}
}

func TestComparatorNoRX(t *testing.T) {
	// 测试没有对应 RX 的情况
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0x01})
	// 没有 RX

	c := NewComparator(golden)

	result := c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})
	if result.Result != Match {
		t.Errorf("result.Result = %v, want Match", result.Result)
	}
	if result.ExpectedRX != nil {
		t.Errorf("ExpectedRX should be nil when no RX in golden")
	}
}

func TestComparatorProgress(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0x01})
	golden.Add(session.RX, []byte{0x02})
	golden.Add(session.TX, []byte{0x03})
	golden.Add(session.RX, []byte{0x04})

	c := NewComparator(golden)

	current, total := c.Progress()
	if current != 0 || total != 2 {
		t.Errorf("Progress() = (%d, %d), want (0, 2)", current, total)
	}

	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})
	current, total = c.Progress()
	if current != 1 || total != 2 {
		t.Errorf("Progress() = (%d, %d), want (1, 2)", current, total)
	}
}

func TestComparatorReset(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0x01})

	c := NewComparator(golden)
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})

	current, _ := c.Progress()
	if current != 1 {
		t.Errorf("Progress() current = %d, want 1", current)
	}

	c.Reset()

	current, _ = c.Progress()
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