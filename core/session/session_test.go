package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSession_New(t *testing.T) {
	s := NewSession("test-device", 115200)
	if s.Device != "test-device" {
		t.Errorf("Device mismatch: got %s, want test-device", s.Device)
	}
	if s.BaudRate != 115200 {
		t.Errorf("BaudRate mismatch: got %d, want 115200", s.BaudRate)
	}
	if len(s.Records) != 0 {
		t.Errorf("Records should be empty, got %d", len(s.Records))
	}
}

func TestSession_Add(t *testing.T) {
	s := NewSession("test", 0)

	// 添加 TX 记录
	s.Add(TX, []byte{0x01, 0x02, 0x03})
	if len(s.Records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(s.Records))
	}
	if s.Records[0].Direction != TX {
		t.Errorf("Direction mismatch: got %s, want TX", s.Records[0].Direction)
	}
	if s.Records[0].Index != 1 {
		t.Errorf("Index should be 1, got %d", s.Records[0].Index)
	}

	// 添加 RX 记录
	s.Add(RX, []byte{0x04, 0x05})
	if len(s.Records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(s.Records))
	}
	if s.Records[1].Direction != RX {
		t.Errorf("Direction mismatch: got %s, want RX", s.Records[1].Direction)
	}
	if s.Records[1].Index != 2 {
		t.Errorf("Index should be 2, got %d", s.Records[1].Index)
	}
}

func TestSession_SaveLoad(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "session-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建并保存会话
	s1 := NewSession("test-device", 9600)
	s1.Add(TX, []byte{0x01, 0x02, 0x03})
	s1.Add(RX, []byte{0x04, 0x05})
	s1.Add(TX, []byte{0x06})

	path := filepath.Join(tmpDir, "test.json")
	err = s1.Save(path)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// 加载会话
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// 验证
	if s2.Device != s1.Device {
		t.Errorf("Device mismatch: got %s, want %s", s2.Device, s1.Device)
	}
	if s2.BaudRate != s1.BaudRate {
		t.Errorf("BaudRate mismatch: got %d, want %d", s2.BaudRate, s1.BaudRate)
	}
	if len(s2.Records) != len(s1.Records) {
		t.Fatalf("Records count mismatch: got %d, want %d", len(s2.Records), len(s1.Records))
	}

	for i, r := range s2.Records {
		if r.Direction != s1.Records[i].Direction {
			t.Errorf("Record %d direction mismatch", i)
		}
		if len(r.Data) != len(s1.Records[i].Data) {
			t.Errorf("Record %d data length mismatch", i)
		}
	}
}

func TestSession_GetStats(t *testing.T) {
	s := NewSession("test", 0)

	// 空会话
	stats := s.GetStats()
	if stats.Total != 0 || stats.TXCount != 0 || stats.RXCount != 0 {
		t.Errorf("Empty session stats should be zero: %+v", stats)
	}

	// 添加记录
	s.Add(TX, []byte{0x01})
	s.Add(RX, []byte{0x02})
	s.Add(TX, []byte{0x03})

	stats = s.GetStats()
	if stats.Total != 3 {
		t.Errorf("Total should be 3, got %d", stats.Total)
	}
	if stats.TXCount != 2 {
		t.Errorf("TXCount should be 2, got %d", stats.TXCount)
	}
	if stats.RXCount != 1 {
		t.Errorf("RXCount should be 1, got %d", stats.RXCount)
	}
}

func TestSession_Clear(t *testing.T) {
	s := NewSession("test", 0)
	s.Add(TX, []byte{0x01})
	s.Add(RX, []byte{0x02})

	s.Clear()

	if len(s.Records) != 0 {
		t.Errorf("Records should be empty after clear, got %d", len(s.Records))
	}
}
