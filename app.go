package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/burnscope-io/burnscope/internal/comparator"
	"github.com/burnscope-io/burnscope/internal/session"
	"github.com/burnscope-io/burnscope/internal/transport"
)

// App 应用状态
type App struct {
	ctx context.Context

	// 串口
	pty1   *transport.PtyTransport  // 代理串口（烧录工具连接）
	pty2   *transport.PtyTransport  // 虚拟串口（测试模式）
	serial *transport.SerialTransport // 真实串口（录制模式）

	// 会话
	golden    *session.Session
	comparator *comparator.Comparator

	mu        sync.Mutex
	isRunning bool
	stopChan  chan struct{}
}

// NewApp 创建应用
func NewApp() *App {
	return &App{
		stopChan: make(chan struct{}),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ListSerialPorts 列出可用串口
func (a *App) ListSerialPorts() ([]string, error) {
	return transport.ListPorts()
}

// ==================== 录制模式（真实设备） ====================

// StartRecord 启动录制模式
// devicePort: 真实设备串口路径（可选，不填则等待自动连接）
// 返回代理串口路径供烧录工具连接
func (a *App) StartRecord(devicePort string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return "", fmt.Errorf("already running")
	}

	// 创建代理串口
	pty1, err := transport.NewPtyTransport()
	if err != nil {
		return "", fmt.Errorf("create proxy port failed: %w", err)
	}

	a.pty1 = pty1
	a.isRunning = true
	a.stopChan = make(chan struct{})
	a.golden = session.NewSession("", 0)

	// 如果指定了真实串口，延迟连接（等待波特率设置）
	if devicePort != "" {
		go func() {
			// 等待波特率或超时
			select {
			case baud := <-pty1.BaudChange():
				// 获取到波特率，连接真实串口
				serial, err := transport.NewSerialTransport(devicePort, baud)
				if err != nil {
					runtime.EventsEmit(a.ctx, "error", fmt.Sprintf("connect device failed: %v", err))
					return
				}
				a.mu.Lock()
				a.serial = serial
				a.golden = session.NewSession(devicePort, baud)
				a.mu.Unlock()
				
				runtime.EventsEmit(a.ctx, "connected", map[string]interface{}{
					"device": devicePort,
					"baud":   baud,
				})
				
				// 启动桥接
				go a.bridgeLoop(a.pty1, serial)
				
			case <-time.After(30 * time.Second):
				runtime.EventsEmit(a.ctx, "error", "timeout waiting for baud rate")
			case <-a.stopChan:
				return
			}
		}()
	}

	return pty1.SlavePath(), nil
}

// ConnectDevice 连接真实设备（手动指定波特率）
func (a *App) ConnectDevice(devicePort string, baud int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.serial != nil {
		return fmt.Errorf("device already connected")
	}

	serial, err := transport.NewSerialTransport(devicePort, baud)
	if err != nil {
		return fmt.Errorf("connect device failed: %w", err)
	}

	a.serial = serial
	a.golden = session.NewSession(devicePort, baud)

	runtime.EventsEmit(a.ctx, "connected", map[string]interface{}{
		"device": devicePort,
		"baud":   baud,
	})

	// 启动桥接
	go a.bridgeLoop(a.pty1, serial)

	return nil
}

// bridgeLoop 双向桥接
func (a *App) bridgeLoop(pty *transport.PtyTransport, target interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}) {
	recordChan := make(chan recordEvent, 1000)
	doneChan := make(chan error, 2)

	// PTY → Target (TX)
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-a.stopChan:
				doneChan <- nil
				return
			default:
				n, err := pty.Read(buf)
				if err != nil {
					doneChan <- err
					return
				}
				if n > 0 {
					target.Write(buf[:n])
					data := make([]byte, n)
					copy(data, buf[:n])
					recordChan <- recordEvent{dir: session.TX, data: data}
				}
			}
		}
	}()

	// Target → PTY (RX)
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-a.stopChan:
				doneChan <- nil
				return
			default:
				n, err := target.Read(buf)
				if err != nil {
					doneChan <- err
					return
				}
				if n > 0 {
					pty.Write(buf[:n])
					data := make([]byte, n)
					copy(data, buf[:n])
					recordChan <- recordEvent{dir: session.RX, data: data}
				}
			}
		}
	}()

	for {
		select {
		case <-a.stopChan:
			return
		case err := <-doneChan:
			if err != nil && err != io.EOF {
				runtime.EventsEmit(a.ctx, "error", err.Error())
			}
			return
		case evt := <-recordChan:
			a.mu.Lock()
			a.golden.Add(evt.dir, evt.data)
			a.mu.Unlock()

			runtime.EventsEmit(a.ctx, "record", map[string]interface{}{
				"direction": string(evt.dir),
				"data":      hex.EncodeToString(evt.data),
				"size":      len(evt.data),
			})
		}
	}
}

type recordEvent struct {
	dir  session.Direction
	data []byte
}

// ==================== 测试模式（双虚拟串口） ====================

// StartTest 启动测试模式
// 返回: (代理串口, 测试下位串口)
func (a *App) StartTest() (string, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return "", "", fmt.Errorf("already running")
	}

	// 创建两个 PTY
	pty1, err := transport.NewPtyTransport()
	if err != nil {
		return "", "", fmt.Errorf("create proxy port failed: %w", err)
	}

	pty2, err := transport.NewPtyTransport()
	if err != nil {
		pty1.Close()
		return "", "", fmt.Errorf("create test port failed: %w", err)
	}

	a.pty1 = pty1
	a.pty2 = pty2
	a.isRunning = true
	a.stopChan = make(chan struct{})

	// 启动双向桥接
	go a.bridgeLoop(pty1, pty2)

	return pty1.SlavePath(), pty2.SlavePath(), nil
}

// ==================== 对比模式 ====================

// StartCompare 启动对比模式
func (a *App) StartCompare() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return "", fmt.Errorf("already running")
	}

	if a.golden == nil || len(a.golden.Records) == 0 {
		return "", fmt.Errorf("no golden session loaded")
	}

	pty, err := transport.NewPtyTransport()
	if err != nil {
		return "", err
	}

	a.pty1 = pty
	a.comparator = comparator.NewComparator(a.golden)
	a.isRunning = true
	a.stopChan = make(chan struct{})

	go a.compareLoop()

	return pty.SlavePath(), nil
}

// compareLoop 对比循环
func (a *App) compareLoop() {
	buf := make([]byte, 4096)

	for {
		select {
		case <-a.stopChan:
			return
		default:
			n, err := a.pty1.Read(buf)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				actual := &session.Record{
					Direction: session.TX,
					Data:      data,
				}

				result := a.comparator.Compare(actual)

				runtime.EventsEmit(a.ctx, "compare", map[string]interface{}{
					"index":    result.Index,
					"expected": formatRecord(result.ExpectedTX),
					"actual":   formatRecord(actual),
					"match":    result.Result == comparator.Match,
				})

				// 回放 RX
				if result.ExpectedRX != nil {
					a.pty1.Write(result.ExpectedRX.Data)
					runtime.EventsEmit(a.ctx, "replay", map[string]interface{}{
						"direction": "RX",
						"data":      hex.EncodeToString(result.ExpectedRX.Data),
					})
				}

				matched, diff, _ := a.comparator.Stats()
				runtime.EventsEmit(a.ctx, "stats", map[string]int{
					"matched": matched,
					"diff":    diff,
				})
			}
		}
	}
}

func formatRecord(r *session.Record) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"index":     r.Index,
		"direction": string(r.Direction),
		"data":      hex.EncodeToString(r.Data),
	}
}

// ==================== 通用操作 ====================

// Stop 停止当前操作
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return nil
	}

	close(a.stopChan)
	a.isRunning = false

	if a.pty1 != nil {
		a.pty1.Close()
		a.pty1 = nil
	}
	if a.pty2 != nil {
		a.pty2.Close()
		a.pty2 = nil
	}
	if a.serial != nil {
		a.serial.Close()
		a.serial = nil
	}

	return nil
}

// SaveSession 保存会话
func (a *App) SaveSession(path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
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
	a.mu.Lock()
	a.golden = s
	a.comparator = comparator.NewComparator(a.golden)
	a.mu.Unlock()
	return nil
}

// GetRecords 获取记录列表
func (a *App) GetRecords() []map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.golden == nil {
		return nil
	}

	records := make([]map[string]interface{}, len(a.golden.Records))
	for i, r := range a.golden.Records {
		records[i] = map[string]interface{}{
			"index":     r.Index,
			"direction": string(r.Direction),
			"data":      hex.EncodeToString(r.Data),
		}
	}
	return records
}

// GetStats 获取统计
func (a *App) GetStats() (matched, diff, total int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.comparator != nil {
		return a.comparator.Stats()
	}
	return 0, 0, 0
}

// GetBaudRate 获取当前波特率（自动捕获）
func (a *App) GetBaudRate() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pty1 != nil {
		return a.pty1.GetBaudRate()
	}
	return 0
}
