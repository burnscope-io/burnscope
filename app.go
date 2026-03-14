package main

import (
	"context"
	"fmt"

	"github.com/burnscope-io/burnscope/internal/comparator"
	"github.com/burnscope-io/burnscope/internal/protocol"
	"github.com/burnscope-io/burnscope/internal/protocol/esp"
	"github.com/burnscope-io/burnscope/internal/session"
	"github.com/burnscope-io/burnscope/internal/transport"
)

// App 应用状态
type App struct {
	ctx       context.Context
	parser    protocol.Parser
	golden    *session.Session
	comparator *comparator.Comparator
	pty       *transport.PtyTransport
	serial    *transport.SerialTransport
}

// NewApp 创建应用
func NewApp() *App {
	return &App{
		parser: esp.NewESPFlashProtocol(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ListSerialPorts 列出可用串口
func (a *App) ListSerialPorts() ([]string, error) {
	return transport.ListPorts()
}

// StartRecording 开始录制
func (a *App) StartRecording(port string, baud int) error {
	var err error
	a.serial, err = transport.NewSerialTransport(port, baud)
	if err != nil {
		return err
	}

	a.golden = session.NewSession(port, baud, a.parser.Name())
	return nil
}

// StopRecording 停止录制
func (a *App) StopRecording() error {
	if a.serial != nil {
		err := a.serial.Close()
		a.serial = nil
		return err
	}
	return nil
}

// SaveSession 保存会话
func (a *App) SaveSession(path string) error {
	if a.golden == nil {
		return fmt.Errorf("no session to save")
	}
	return a.golden.Save(path)
}

// LoadSession 加载会话
func (a *App) LoadSession(path string) error {
	s, err := session.Load(path)
	if err != nil {
		return err
	}
	a.golden = s
	a.comparator = comparator.NewComparator(a.golden)
	return nil
}

// StartCompare 开始对比
func (a *App) StartCompare() (string, error) {
	var err error
	a.pty, err = transport.NewPtyTransport()
	if err != nil {
		return "", err
	}

	if a.golden == nil {
		return "", fmt.Errorf("no golden session loaded")
	}

	a.comparator = comparator.NewComparator(a.golden)
	return a.pty.SlavePath(), nil
}

// StopCompare 停止对比
func (a *App) StopCompare() error {
	if a.pty != nil {
		err := a.pty.Close()
		a.pty = nil
		return err
	}
	return nil
}

// GetStats 获取统计
func (a *App) GetStats() (matched, diff, total int) {
	if a.comparator != nil {
		return a.comparator.Stats()
	}
	return 0, 0, 0
}

// GetRecords 获取记录列表
func (a *App) GetRecords() []session.Record {
	if a.golden == nil {
		return nil
	}
	return a.golden.Records
}
