package comparator

import (
	"testing"

	"github.com/burnscope-io/burnscope/internal/session"
)

func TestComparatorMatch(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0xC0, 0x00, 0x08, 0xC0})
	golden.Add(session.RX, []byte{0xC0, 0x01, 0x08, 0xC0})

	c := NewComparator(golden)

	// 第一条：匹配
	result := c.Compare(&session.Record{Direction: session.TX, Data: []byte{0xC0, 0x00, 0x08, 0xC0}})
	if result.Result != Match {
		t.Errorf("result.Result = %v, want Match", result.Result)
	}

	// 第二条：匹配
	result = c.Compare(&session.Record{Direction: session.RX, Data: []byte{0xC0, 0x01, 0x08, 0xC0}})
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
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0xC0, 0x00, 0x08, 0xC0})

	c := NewComparator(golden)

	// 方向不同
	result := c.Compare(&session.Record{Direction: session.RX, Data: []byte{0xC0, 0x00, 0x08, 0xC0}})
	if result.Result != Diff {
		t.Errorf("result.Result = %v, want Diff for direction mismatch", result.Result)
	}

	c.Reset()

	// 数据不同
	result = c.Compare(&session.Record{Direction: session.TX, Data: []byte{0xC0, 0x00, 0x09, 0xC0}})
	if result.Result != Diff {
		t.Errorf("result.Result = %v, want Diff for data mismatch", result.Result)
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

func TestComparatorProgress(t *testing.T) {
	golden := session.NewSession("/dev/ttyUSB0", 115200)
	golden.Add(session.TX, []byte{0x01})
	golden.Add(session.RX, []byte{0x02})
	golden.Add(session.TX, []byte{0x03})

	c := NewComparator(golden)

	current, total := c.Progress()
	if current != 0 || total != 3 {
		t.Errorf("Progress() = (%d, %d), want (0, 3)", current, total)
	}

	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})
	current, total = c.Progress()
	if current != 1 || total != 3 {
		t.Errorf("Progress() = (%d, %d), want (1, 3)", current, total)
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
