//go:build integration

package main

import (
	"context"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/burnscope-io/burnscope/core/api"
)

// TestAppIntegration_Init 测试应用初始化（默认录制模式）
func TestAppIntegration_Init(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	state := app.Init()

	// Init now auto-starts record mode
	if state.Mode != string(api.ModeRecord) {
		t.Errorf("Mode should be record, got %s", state.Mode)
	}

	if state.UpperPort == "" {
		t.Error("UpperPort should be set")
	}

	t.Logf("UpperPort: %s", state.UpperPort)
	t.Logf("LowerPorts: %d", len(state.LowerPorts))

	// Cleanup
	app.Stop()
}

// TestAppIntegration_RecordCycle 测试录制周期
func TestAppIntegration_RecordCycle(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// Init (auto-starts record mode)
	state := app.Init()

	if state.Mode != string(api.ModeRecord) {
		t.Errorf("Mode should be record, got %s", state.Mode)
	}

	// Simulate data by writing to the port
	if state.UpperPort != "" {
		slave, err := os.OpenFile(state.UpperPort, os.O_RDWR, 0)
		if err == nil {
			testData := []byte{0xC0, 0x00, 0x08, 0x24}
			slave.Write(testData)
			time.Sleep(100 * time.Millisecond)
			slave.Close()
		}
	}

	// Stop
	state = app.Stop()
	t.Logf("Baseline records: %d", len(state.Baseline))

	// Clear (returns to record mode)
	state = app.Clear()
	if len(state.Baseline) != 0 {
		t.Error("Baseline should be empty after clear")
	}
	if state.Mode != string(api.ModeRecord) {
		t.Error("Mode should be record after clear")
	}
}

// TestAppIntegration_CompareCycle 测试对比周期
func TestAppIntegration_CompareCycle(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// Init (auto-starts record mode)
	state := app.Init()

	// Stop record mode first
	app.Stop()

	// Set baseline manually for test
	baseline := []api.Record{
		{Index: 0, Dir: "TX", Data: hex.EncodeToString([]byte{0xC0, 0x00}), Size: 2},
		{Index: 1, Dir: "RX", Data: hex.EncodeToString([]byte{0xC0, 0x01}), Size: 2},
	}
	app.service.SetBaseline(baseline)

	// Start Compare
	state = app.StartCompare()
	if state.Mode != string(api.ModeCompare) {
		t.Errorf("Mode should be compare, got %s", state.Mode)
	}

	t.Logf("Compare mode started, baseline: %d", len(state.Baseline))

	// Stop
	state = app.Stop()

	// Clear
	app.Clear()
}

// TestAppIntegration_StateConsistency 测试状态一致性
func TestAppIntegration_StateConsistency(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// 多次获取状态应该一致
	app.Init()

	state1 := app.GetState()
	state2 := app.GetState()

	if state1.Mode != state2.Mode {
		t.Error("State should be consistent")
	}

	if state1.UpperPort != state2.UpperPort {
		t.Error("UpperPort should be consistent")
	}

	app.Stop()
}

// TestAppIntegration_RefreshPorts 测试端口刷新
func TestAppIntegration_RefreshPorts(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	app.Init()

	state := app.RefreshPorts()

	if len(state.LowerPorts) == 0 {
		t.Error("Should have at least one port after refresh")
	}

	t.Logf("Ports after refresh: %d", len(state.LowerPorts))

	app.Stop()
}

// TestAppIntegration_FullWorkflow 测试完整工作流
func TestAppIntegration_FullWorkflow(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// 1. 初始化（自动开始录制）
	state := app.Init()
	t.Logf("1. Init: mode=%s, upper=%s", state.Mode, state.UpperPort)

	if state.Mode != string(api.ModeRecord) {
		t.Fatal("Should be in record mode after init")
	}

	// 2. 模拟数据
	if state.UpperPort != "" {
		slave, _ := os.OpenFile(state.UpperPort, os.O_RDWR, 0)
		if slave != nil {
			slave.Write([]byte{0xC0, 0x00})
			time.Sleep(50 * time.Millisecond)
			slave.Close()
		}
	}

	// 3. 停止录制
	state = app.Stop()
	t.Logf("2. Stop: mode=%s, baseline=%d", state.Mode, len(state.Baseline))

	// 4. 清空（返回录制模式）
	state = app.Clear()
	t.Logf("3. Clear: mode=%s, baseline=%d", state.Mode, len(state.Baseline))

	if state.Mode != string(api.ModeRecord) {
		t.Error("Should be record mode after clear")
	}
}