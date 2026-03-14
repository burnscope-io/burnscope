package main

import (
	"testing"

	"github.com/burnscope-io/burnscope/internal/transport"
)

func TestInitPorts(t *testing.T) {
	app := NewApp()

	// 测试初始化端口
	ports := app.InitPorts()

	if ports == nil {
		// PTY 创建可能失败（非终端环境）
		t.Skip("InitPorts returned nil (PTY creation may fail in non-terminal env)")
	}

	if ports["upper"] == "" {
		t.Error("upper port is empty")
	}

	if ports["lower"] == "" {
		t.Error("lower port is empty")
	}

	t.Logf("upper: %s", ports["upper"])
	t.Logf("lower: %s", ports["lower"])

	// 再次调用应该返回相同结果（幂等）
	ports2 := app.InitPorts()
	if ports2["upper"] != ports["upper"] {
		t.Error("upper port changed on second call")
	}
	if ports2["lower"] != ports["lower"] {
		t.Error("lower port changed on second call")
	}
}

func TestGetPorts(t *testing.T) {
	app := NewApp()

	// 尝试初始化
	ports := app.InitPorts()
	if ports == nil {
		t.Skip("PTY creation failed in non-terminal env")
	}

	if app.GetUpperPort() == "" {
		t.Error("upper port is empty after init")
	}

	if app.GetLowerPort() == "" {
		t.Error("lower port is empty after init")
	}
}

func TestPtyCreation(t *testing.T) {
	// 直接测试 PTY 创建
	pty, err := transport.NewPtyTransport()
	if err != nil {
		// PTY 创建需要终端环境，在非终端环境跳过
		t.Skipf("NewPtyTransport failed (expected in non-terminal env): %v", err)
	}
	defer pty.Close()

	slavePath := pty.SlavePath()
	if slavePath == "" {
		t.Error("SlavePath is empty")
	}

	t.Logf("PTY slave: %s", slavePath)
}
