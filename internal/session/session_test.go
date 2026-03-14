package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	s := NewSession("/dev/ttyUSB0", 115200)

	if s.Device != "/dev/ttyUSB0" {
		t.Errorf("s.Device = %s, want /dev/ttyUSB0", s.Device)
	}
	if s.BaudRate != 115200 {
		t.Errorf("s.BaudRate = %d, want 115200", s.BaudRate)
	}
	if len(s.Records) != 0 {
		t.Errorf("len(s.Records) = %d, want 0", len(s.Records))
	}
}

func TestAdd(t *testing.T) {
	s := NewSession("/dev/ttyUSB0", 115200)

	s.Add(TX, []byte{0x01, 0x02, 0x03})
	s.Add(RX, []byte{0x04, 0x05})

	if len(s.Records) != 2 {
		t.Fatalf("len(s.Records) = %d, want 2", len(s.Records))
	}

	if s.Records[0].Direction != TX {
		t.Errorf("s.Records[0].Direction = %s, want TX", s.Records[0].Direction)
	}
	if s.Records[1].Direction != RX {
		t.Errorf("s.Records[1].Direction = %s, want RX", s.Records[1].Direction)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.json")

	s := NewSession("/dev/ttyUSB0", 115200)
	s.Add(TX, []byte{0x01})
	s.Add(RX, []byte{0x02})

	if err := s.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Device != s.Device {
		t.Errorf("loaded.Device = %s, want %s", loaded.Device, s.Device)
	}
	if loaded.BaudRate != s.BaudRate {
		t.Errorf("loaded.BaudRate = %d, want %d", loaded.BaudRate, s.BaudRate)
	}
	if len(loaded.Records) != len(s.Records) {
		t.Errorf("len(loaded.Records) = %d, want %d", len(loaded.Records), len(s.Records))
	}
}

func TestGetStats(t *testing.T) {
	s := NewSession("/dev/ttyUSB0", 115200)

	s.Add(TX, []byte{0x01})
	s.Add(RX, []byte{0x02})
	s.Add(TX, []byte{0x03})

	stats := s.GetStats()

	if stats.Total != 3 {
		t.Errorf("stats.Total = %d, want 3", stats.Total)
	}
	if stats.TXCount != 2 {
		t.Errorf("stats.TXCount = %d, want 2", stats.TXCount)
	}
	if stats.RXCount != 1 {
		t.Errorf("stats.RXCount = %d, want 1", stats.RXCount)
	}
}

func TestClear(t *testing.T) {
	s := NewSession("/dev/ttyUSB0", 115200)

	s.Add(TX, []byte{0x01})
	s.Add(RX, []byte{0x02})

	s.Clear()

	if len(s.Records) != 0 {
		t.Errorf("len(s.Records) = %d, want 0 after Clear()", len(s.Records))
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/session.json")
	if err == nil {
		t.Error("Load() should return error for non-existent file")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "session.json")

	s := NewSession("/dev/ttyUSB0", 115200)

	err := s.Save(path)
	if err == nil {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file should exist after Save()")
		}
	}
}

func TestRecordTimestamp(t *testing.T) {
	s := NewSession("/dev/ttyUSB0", 115200)

	before := time.Now()
	s.Add(TX, []byte{0x01})
	after := time.Now()

	if s.Records[0].Timestamp.Before(before) || s.Records[0].Timestamp.After(after) {
		t.Errorf("Timestamp = %v, should be between %v and %v",
			s.Records[0].Timestamp, before, after)
	}
}