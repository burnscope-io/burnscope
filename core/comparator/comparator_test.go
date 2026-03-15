package comparator

import (
	"testing"

	"github.com/burnscope-io/burnscope/core/session"
)

func TestComparator_Match(t *testing.T) {
	// 创建基准会话
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01, 0x02, 0x03})
	golden.Add(session.RX, []byte{0x04, 0x05})
	golden.Add(session.TX, []byte{0x06, 0x07})

	c := NewComparator(golden)

	// 对比第一条 TX
	result := c.Compare(&session.Record{
		Direction: session.TX,
		Data:      []byte{0x01, 0x02, 0x03},
	})

	if result.Result != Match {
		t.Errorf("Expected Match, got %v", result.Result)
	}
	if result.Index != 0 {
		t.Errorf("Expected index 0, got %d", result.Index)
	}

	// 对比第二条 TX
	result = c.Compare(&session.Record{
		Direction: session.TX,
		Data:      []byte{0x06, 0x07},
	})

	if result.Result != Match {
		t.Errorf("Expected Match, got %v", result.Result)
	}
	if result.Index != 1 {
		t.Errorf("Expected index 1, got %d", result.Index)
	}
}

func TestComparator_Diff(t *testing.T) {
	// 创建基准会话
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01, 0x02, 0x03})
	golden.Add(session.TX, []byte{0x06, 0x07})

	c := NewComparator(golden)

	// 对比不同的数据
	result := c.Compare(&session.Record{
		Direction: session.TX,
		Data:      []byte{0x01, 0x02, 0xFF}, // 最后一个字节不同
	})

	if result.Result != Diff {
		t.Errorf("Expected Diff, got %v", result.Result)
	}
}

func TestComparator_Stats(t *testing.T) {
	// 创建基准会话
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01})
	golden.Add(session.TX, []byte{0x02})
	golden.Add(session.TX, []byte{0x03})

	c := NewComparator(golden)

	// 第一次匹配
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})
	// 第二次不匹配
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0xFF}})
	// 第三次匹配
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x03}})

	matched, diff, total := c.Stats()
	if matched != 2 {
		t.Errorf("Expected 2 matched, got %d", matched)
	}
	if diff != 1 {
		t.Errorf("Expected 1 diff, got %d", diff)
	}
	if total != 3 {
		t.Errorf("Expected 3 total, got %d", total)
	}
}

func TestComparator_Reset(t *testing.T) {
	// 创建基准会话
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01})
	golden.Add(session.TX, []byte{0x02})

	c := NewComparator(golden)

	// 对比一条
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})

	// 重置
	c.Reset()

	// 验证状态
	matched, diff, total := c.Stats()
	if matched != 0 || diff != 0 || total != 0 {
		t.Errorf("After reset, stats should be zero: matched=%d, diff=%d, total=%d", matched, diff, total)
	}
}

func TestComparator_Progress(t *testing.T) {
	// 创建基准会话
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01})
	golden.Add(session.RX, []byte{0x02}) // RX 不计入 TX 进度
	golden.Add(session.TX, []byte{0x03})
	golden.Add(session.TX, []byte{0x04})

	c := NewComparator(golden)

	current, total := c.Progress()
	if current != 0 {
		t.Errorf("Expected current 0, got %d", current)
	}
	if total != 3 {
		t.Errorf("Expected total 3 (TX count), got %d", total)
	}

	// 对比一条
	c.Compare(&session.Record{Direction: session.TX, Data: []byte{0x01}})

	current, total = c.Progress()
	if current != 1 {
		t.Errorf("Expected current 1, got %d", current)
	}
}

func TestComparator_ExpectedRX(t *testing.T) {
	// 创建基准会话: TX -> RX -> TX
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01, 0x02})
	golden.Add(session.RX, []byte{0x03, 0x04}) // 对应第一条 TX 的响应
	golden.Add(session.TX, []byte{0x05})

	c := NewComparator(golden)

	// 对比第一条 TX
	result := c.Compare(&session.Record{
		Direction: session.TX,
		Data:      []byte{0x01, 0x02},
	})

	if result.Result != Match {
		t.Errorf("Expected Match, got %v", result.Result)
	}
	if len(result.ExpectedRXs) == 0 {
		t.Error("ExpectedRXs should not be empty")
	} else {
		if len(result.ExpectedRXs[0].Data) != 2 {
			t.Errorf("Expected RX data length 2, got %d", len(result.ExpectedRXs[0].Data))
		}
	}
}

func TestComparator_MultipleRXs(t *testing.T) {
	// 创建基准会话: TX -> RX -> RX -> TX
	golden := session.NewSession("test", 0)
	golden.Add(session.TX, []byte{0x01})
	golden.Add(session.RX, []byte{0x02}) // 第一个 RX
	golden.Add(session.RX, []byte{0x03}) // 第二个 RX（连续）
	golden.Add(session.TX, []byte{0x04})

	c := NewComparator(golden)

	// 对比第一条 TX
	result := c.Compare(&session.Record{
		Direction: session.TX,
		Data:      []byte{0x01},
	})

	if result.Result != Match {
		t.Errorf("Expected Match, got %v", result.Result)
	}
	if len(result.ExpectedRXs) != 2 {
		t.Errorf("Expected 2 RXs, got %d", len(result.ExpectedRXs))
	}
}
