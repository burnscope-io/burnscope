//go:build integration

package service_test

import (
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/burnscope-io/burnscope/core/api"
	"github.com/burnscope-io/burnscope/core/service"
	"github.com/burnscope-io/burnscope/core/session"
	"github.com/burnscope-io/burnscope/core/transport"
)

// TestIntegration_RecordFlow 测试录制流程集成
func TestIntegration_RecordFlow(t *testing.T) {
	svc := service.NewService()

	// 初始化（自动开始录制）
	state, err := svc.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if state.Mode != string(api.ModeRecord) {
		t.Errorf("Mode should be record after Init, got %s", state.Mode)
	}

	if state.UpperPort == "" {
		t.Error("UpperPort should be set after Init")
	}

	t.Logf("Upper: %s", state.UpperPort)
	t.Logf("Lower ports: %d", len(state.LowerPorts))

	// 获取端口
	upperPort := state.UpperPort

	// 打开 slave 进行写入测试
	slave, err := os.OpenFile(upperPort, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Failed to open slave: %v", err)
	}
	defer slave.Close()

	// 模拟烧录工具发送数据
	testData := []byte{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00}
	n, err := slave.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch: %d != %d", n, len(testData))
	}

	// 等待数据被处理
	time.Sleep(100 * time.Millisecond)

	// 停止录制
	state = svc.Stop()
	if state.Mode != string(api.ModeIdle) {
		t.Errorf("Mode should be idle after stop, got %s", state.Mode)
	}

	t.Logf("Baseline records: %d", len(state.Baseline))

	// 清理
	svc.Clear()
}

// TestIntegration_CompareFlow 测试对比流程集成
func TestIntegration_CompareFlow(t *testing.T) {
	svc := service.NewService()

	// 初始化
	_, err := svc.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 停止录制模式
	svc.Stop()

	// 准备基准数据
	baseline := []api.Record{
		{Index: 0, Dir: "TX", Data: hex.EncodeToString([]byte{0xC0, 0x00}), Size: 2},
		{Index: 1, Dir: "RX", Data: hex.EncodeToString([]byte{0xC0, 0x01}), Size: 2},
	}

	// 直接设置基准数据
	svc.SetBaseline(baseline)

	// 开始对比
	state, err := svc.StartCompare()
	if err != nil {
		t.Fatalf("StartCompare failed: %v", err)
	}

	if state.Mode != string(api.ModeCompare) {
		t.Errorf("Mode should be compare, got %s", state.Mode)
	}

	// 停止
	state = svc.Stop()
	if state.Mode != string(api.ModeIdle) {
		t.Errorf("Mode should be idle, got %s", state.Mode)
	}

	svc.Clear()
}

// TestIntegration_FullCycle 测试完整录制-对比周期
func TestIntegration_FullCycle(t *testing.T) {
	svc := service.NewService()

	// === 阶段1: 录制（Init 自动开始）===
	state, err := svc.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if state.Mode != string(api.ModeRecord) {
		t.Errorf("Mode should be record after Init, got %s", state.Mode)
	}

	// 模拟数据流
	slave, err := os.OpenFile(state.UpperPort, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Open slave failed: %v", err)
	}

	// 发送测试数据
	testData := []byte{0xC0, 0x00, 0x08, 0x24}
	slave.Write(testData)

	time.Sleep(100 * time.Millisecond)
	slave.Close()

	// 停止录制
	state = svc.Stop()
	t.Logf("Recorded %d baseline records", len(state.Baseline))

	// === 阶段2: 对比 ===
	if len(state.Baseline) > 0 {
		state, err = svc.StartCompare()
		if err != nil {
			t.Fatalf("StartCompare failed: %v", err)
		}

		t.Logf("Compare mode started, baseline: %d", len(state.Baseline))

		state = svc.Stop()
	}

	// === 清理 ===
	svc.Clear()
}

// TestIntegration_PTYPair 测试 PTY 配对通信
func TestIntegration_PTYPair(t *testing.T) {
	// 创建两个 PTY
	pty1, err := transport.NewPtyTransport()
	if err != nil {
		t.Fatalf("Create PTY1 failed: %v", err)
	}
	defer pty1.Close()

	pty2, err := transport.NewPtyTransport()
	if err != nil {
		t.Fatalf("Create PTY2 failed: %v", err)
	}
	defer pty2.Close()

	t.Logf("PTY1: %s", pty1.SlavePath())
	t.Logf("PTY2: %s", pty2.SlavePath())

	// 打开 slave
	slave1, err := os.OpenFile(pty1.SlavePath(), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("Open slave1 failed: %v", err)
	}
	defer slave1.Close()

	// 测试写入
	testData := []byte("hello pty")
	n, err := pty1.Write(testData)
	if err != nil {
		t.Fatalf("PTY1 write failed: %v", err)
	}

	buf := make([]byte, 64)
	n, err = slave1.Read(buf)
	if err != nil {
		t.Fatalf("Slave1 read failed: %v", err)
	}
	if string(buf[:n]) != string(testData) {
		t.Errorf("Data mismatch: got %q, want %q", buf[:n], testData)
	}

	t.Logf("PTY pair test passed, %d bytes transferred", n)
}

// TestIntegration_SessionPersistence 测试会话持久化
func TestIntegration_SessionPersistence(t *testing.T) {
	// 创建会话
	sess := session.NewSession("test-session", 115200)

	// 添加记录
	txData := []byte{0xC0, 0x00, 0x08, 0x24}
	rxData := []byte{0xC0, 0x01, 0x08, 0x24}

	sess.Add(session.TX, txData)
	sess.Add(session.RX, rxData)

	stats := sess.GetStats()
	if stats.TXCount != 1 || stats.RXCount != 1 {
		t.Errorf("Stats mismatch: TX=%d, RX=%d", stats.TXCount, stats.RXCount)
	}

	// 保存
	tmpFile := "/tmp/burnscope-integration-test.golden"
	err := sess.Save(tmpFile)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	defer os.Remove(tmpFile)

	// 加载
	loaded, err := session.Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Records) != 2 {
		t.Errorf("Loaded records mismatch: %d", len(loaded.Records))
	}

	t.Logf("Session persistence test passed, %d records saved/loaded", len(loaded.Records))
}

// TestIntegration_EventFlow 测试事件流
func TestIntegration_EventFlow(t *testing.T) {
	svc := service.NewService()

	receivedEvents := make([]string, 0)
	done := make(chan struct{})

	svc.SetEventCallback(func(event string, data interface{}) {
		receivedEvents = append(receivedEvents, event)
	})

	// 初始化（自动开始录制）
	svc.Init()

	// 等待后停止
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	// 清理
	svc.Clear()

	close(done)

	t.Logf("Received %d events", len(receivedEvents))
}

// TestIntegration_StateTransitions 测试状态转换
func TestIntegration_StateTransitions(t *testing.T) {
	svc := service.NewService()

	// 初始状态
	state := svc.GetState()
	if state.Mode != string(api.ModeIdle) {
		t.Errorf("Initial mode should be idle, got %s", state.Mode)
	}

	// Init（自动开始录制）
	state, _ = svc.Init()
	if state.Mode != string(api.ModeRecord) {
		t.Errorf("After Init, mode should be record, got %s", state.Mode)
	}

	// Stop
	state = svc.Stop()
	if state.Mode != string(api.ModeIdle) {
		t.Errorf("After Stop, mode should be idle, got %s", state.Mode)
	}

	// Clear（返回录制模式）
	state = svc.Clear()
	if state.Mode != string(api.ModeRecord) {
		t.Errorf("After Clear, mode should be record, got %s", state.Mode)
	}
}
