//go:build darwin

package transport

import (
	"os"
	"testing"
	"time"
)

func TestPtyTransport_Create(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	// 检查 slave 路径
	slavePath := pty.SlavePath()
	if slavePath == "" {
		t.Error("Slave path should not be empty")
	}
	t.Logf("Slave path: %s", slavePath)
}

func TestPtyTransport_Name(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	if pty.Name() != "pty" {
		t.Errorf("Name() = %s, want pty", pty.Name())
	}
}

func TestPtyTransport_GetBaudRate(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	// Initial baud rate should be 0 or detected
	baud := pty.GetBaudRate()
	t.Logf("Initial baud rate: %d", baud)
}

func TestPtyTransport_BaudChange(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	// Get baud change channel
	ch := pty.BaudChange()
	if ch == nil {
		t.Error("BaudChange() returned nil channel")
	}

	// Non-blocking check - no change expected initially
	select {
	case baud := <-ch:
		t.Logf("Baud change detected: %d", baud)
	default:
		t.Log("No baud change (expected)")
	}
}

func TestPtyTransport_WriteMaster_ReadSlave(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	// 打开 slave 端
	slave, err := os.OpenFile(pty.SlavePath(), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Failed to open slave: %v", err)
	}
	defer slave.Close()

	// 测试数据
	testData := []byte("hello world\n")

	// 写入 master
	n, err := pty.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write to master: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch: got %d, want %d", n, len(testData))
	}

	// 从 slave 读取
	buf := make([]byte, 1024)
	slave.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = slave.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from slave: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Read length mismatch: got %d, want %d", n, len(testData))
	}
	t.Logf("Read from slave: %q", buf[:n])
}

func TestPtyTransport_WriteSlave_ReadMaster(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	// 打开 slave 端
	slave, err := os.OpenFile(pty.SlavePath(), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Failed to open slave: %v", err)
	}
	defer slave.Close()

	// 测试数据
	testData := []byte("hello world\n")

	// 写入 slave
	n, err := slave.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write to slave: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch: got %d, want %d", n, len(testData))
	}

	// 从 master 读取
	buf := make([]byte, 1024)
	// 等待数据
	time.Sleep(10 * time.Millisecond)
	n, err = pty.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from master: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Read length mismatch: got %d, want %d", n, len(testData))
	}
	t.Logf("Read from master: %q", buf[:n])
}

func TestPtyTransport_MultipleMessages(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	// 打开 slave 端
	slave, err := os.OpenFile(pty.SlavePath(), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Failed to open slave: %v", err)
	}
	defer slave.Close()

	// 发送多条消息
	messages := [][]byte{
		[]byte("msg1\n"),
		[]byte("msg2\n"),
		[]byte("msg3\n"),
	}

	for i, msg := range messages {
		// 写入 slave
		n, err := slave.Write(msg)
		if err != nil {
			t.Fatalf("Failed to write message %d: %v", i, err)
		}
		if n != len(msg) {
			t.Errorf("Write length mismatch for message %d: got %d, want %d", i, n, len(msg))
		}
	}

	// 从 master 读取所有消息
	totalRead := 0
	buf := make([]byte, 1024)
	for totalRead < 15 { // 3 * 5 bytes
		n, err := pty.Read(buf[totalRead:])
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		totalRead += n
		t.Logf("Read %d bytes, total: %d", n, totalRead)
	}

	t.Logf("Total read: %d bytes: %q", totalRead, buf[:totalRead])
}

func TestPtyTransport_Bidirectional(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	slave, err := os.OpenFile(pty.SlavePath(), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Failed to open slave: %v", err)
	}
	defer slave.Close()

	// Write master -> slave
	pty.Write([]byte("master->slave"))
	time.Sleep(10 * time.Millisecond)

	buf := make([]byte, 64)
	n, _ := slave.Read(buf)
	if string(buf[:n]) != "master->slave" {
		t.Errorf("master->slave: got %q", buf[:n])
	}

	// Write slave -> master
	slave.Write([]byte("slave->master"))
	time.Sleep(10 * time.Millisecond)

	n, _ = pty.Read(buf)
	if string(buf[:n]) != "slave->master" {
		t.Errorf("slave->master: got %q", buf[:n])
	}
}

func TestPtyTransport_SmallData(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()

	slave, err := os.OpenFile(pty.SlavePath(), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Failed to open slave: %v", err)
	}
	defer slave.Close()

	// Write small data (256 bytes)
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i % 256)
	}

	n, err := pty.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write length: got %d, want %d", n, len(data))
	}

	// Read all
	buf := make([]byte, 512)
	slave.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err = slave.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Read length: got %d, want %d", n, len(data))
	}
}

func TestPtyTransport_Close(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}

	slavePath := pty.SlavePath()

	// 关闭
	err = pty.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	t.Logf("PTY closed, slave was: %s", slavePath)
}

func TestPtyTransport_DoubleClose(t *testing.T) {
	pty, err := NewPtyTransport()
	if err != nil {
		t.Fatalf("Failed to create PTY: %v", err)
	}

	// Close once
	err = pty.Close()
	if err != nil {
		t.Errorf("First close error: %v", err)
	}

	// Close again - should not panic
	err = pty.Close()
	if err != nil {
		t.Logf("Second close: %v (may be expected)", err)
	}
}