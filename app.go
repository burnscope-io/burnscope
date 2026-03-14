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

	// 两个 PTY
	upperPty *transport.PtyTransport // 上位串口（代理）
	lowerPty *transport.PtyTransport // 下位串口（虚拟）

	// 物理串口
	serial *transport.SerialTransport

	// 会话
	golden    *session.Session
	comparator *comparator.Comparator

	mu        sync.Mutex
	isRunning bool
	stopChan  chan struct{}

	// 初始化状态
	upperPort string
	lowerPort string
}

// NewApp 创建应用
func NewApp() *App {
	return &App{
		stopChan: make(chan struct{}),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	// 启动时立即创建两个 PTY
	a.initPorts()
}

// initPorts 初始化端口
func (a *App) initPorts() {
	// 创建上位串口（代理）
	upper, err := transport.NewPtyTransport()
	if err != nil {
		runtime.EventsEmit(a.ctx, "error", fmt.Sprintf("创建上位串口失败: %v", err))
		return
	}
	a.upperPty = upper
	a.upperPort = upper.SlavePath()

	// 创建下位串口（虚拟）
	lower, err := transport.NewPtyTransport()
	if err != nil {
		runtime.EventsEmit(a.ctx, "error", fmt.Sprintf("创建下位串口失败: %v", err))
		return
	}
	a.lowerPty = lower
	a.lowerPort = lower.SlavePath()

	// 通知前端
	runtime.EventsEmit(a.ctx, "ports_ready", map[string]string{
		"upper": a.upperPort,
		"lower": a.lowerPort,
	})
}

// GetUpperPort 获取上位串口路径
func (a *App) GetUpperPort() string {
	return a.upperPort
}

// GetLowerPort 获取下位串口路径
func (a *App) GetLowerPort() string {
	return a.lowerPort
}

// ListSerialPorts 列出可用物理串口
func (a *App) ListSerialPorts() ([]string, error) {
	return transport.ListPorts()
}

// GetBaudRate 获取捕获的波特率
func (a *App) GetBaudRate() int {
	if a.upperPty != nil {
		return a.upperPty.GetBaudRate()
	}
	return 0
}

// StartSimulate 启动模拟模式（上位PTY ↔ 下位PTY）
func (a *App) StartSimulate() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return fmt.Errorf("already running")
	}

	a.golden = session.NewSession("simulate", 0)
	a.isRunning = true
	a.stopChan = make(chan struct{})

	go a.bridgeLoop(a.upperPty, a.lowerPty)

	return nil
}

// StartRecord 启动录制模式（上位PTY ↔ 物理串口）
func (a *App) StartRecord(port string, baud int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return fmt.Errorf("already running")
	}

	// 连接物理串口
	serial, err := transport.NewSerialTransport(port, baud)
	if err != nil {
		return fmt.Errorf("连接串口失败: %w", err)
	}

	a.serial = serial
	a.golden = session.NewSession(port, baud)
	a.isRunning = true
	a.stopChan = make(chan struct{})

	go a.bridgeLoop(a.upperPty, serial)

	return nil
}

// StartCompare 启动对比模式（需要先加载基准）
func (a *App) StartCompare() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return fmt.Errorf("already running")
	}

	if a.golden == nil || len(a.golden.Records) == 0 {
		return fmt.Errorf("请先加载基准文件")
	}

	a.comparator = comparator.NewComparator(a.golden)
	a.isRunning = true
	a.stopChan = make(chan struct{})

	go a.compareLoop()

	return nil
}

// bridgeLoop 双向桥接
func (a *App) bridgeLoop(upper, lower interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}) {
	recordChan := make(chan recordEvent, 1000)
	doneChan := make(chan error, 2)

	// 上位 → 下位 (TX)
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-a.stopChan:
				doneChan <- nil
				return
			default:
				n, err := upper.Read(buf)
				if err != nil {
					doneChan <- err
					return
				}
				if n > 0 {
					lower.Write(buf[:n])
					data := make([]byte, n)
					copy(data, buf[:n])
					recordChan <- recordEvent{dir: session.TX, data: data}
				}
			}
		}
	}()

	// 下位 → 上位 (RX)
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-a.stopChan:
				doneChan <- nil
				return
			default:
				n, err := lower.Read(buf)
				if err != nil {
					doneChan <- err
					return
				}
				if n > 0 {
					upper.Write(buf[:n])
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

			runtime.EventsEmit(a.ctx, "data", map[string]interface{}{
				"direction": string(evt.dir),
				"data":      hex.EncodeToString(evt.data),
				"size":      len(evt.data),
			})
		}
	}
}

// compareLoop 对比循环
func (a *App) compareLoop() {
	buf := make([]byte, 4096)

	for {
		select {
		case <-a.stopChan:
			return
		default:
			n, err := a.upperPty.Read(buf)
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
					a.upperPty.Write(result.ExpectedRX.Data)
					runtime.EventsEmit(a.ctx, "data", map[string]interface{}{
						"direction": "RX",
						"data":      hex.EncodeToString(result.ExpectedRX.Data),
						"size":      len(result.ExpectedRX.Data),
						"replay":    true,
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

type recordEvent struct {
	dir  session.Direction
	data []byte
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

// Stop 停止
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return nil
	}

	close(a.stopChan)
	a.isRunning = false

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
func (a *App) LoadSession(path string) (int, error) {
	s, err := session.Load(path)
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.golden = s
	a.comparator = comparator.NewComparator(a.golden)
	a.mu.Unlock()
	return len(s.Records), nil
}

// GetStats 获取统计
func (a *App) GetStats() map[string]int {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	stats := map[string]int{
		"tx": 0,
		"rx": 0,
		"matched": 0,
		"diff": 0,
	}
	
	if a.golden != nil {
		s := a.golden.GetStats()
		stats["tx"] = s.TXCount
		stats["rx"] = s.RXCount
	}
	
	if a.comparator != nil {
		matched, diff, _ := a.comparator.Stats()
		stats["matched"] = matched
		stats["diff"] = diff
	}
	
	return stats
}