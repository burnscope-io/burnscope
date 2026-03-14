package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/burnscope-io/burnscope/internal/comparator"
	"github.com/burnscope-io/burnscope/internal/protocol"
	"github.com/burnscope-io/burnscope/internal/protocol/esp"
	"github.com/burnscope-io/burnscope/internal/session"
	"github.com/burnscope-io/burnscope/internal/transport"
)

// App 应用状态
type App struct {
	ctx        context.Context
	parser     protocol.Parser
	golden     *session.Session
	comparator *comparator.Comparator
	pty        *transport.PtyTransport
	serial     *transport.SerialTransport

	// 运行状态
	mu        sync.Mutex
	isRunning bool
	stopChan  chan struct{}

	// 对比模式
	compareIndex int
}

// NewApp 创建应用
func NewApp() *App {
	return &App{
		parser:    esp.NewESPFlashProtocol(),
		stopChan:  make(chan struct{}),
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
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return fmt.Errorf("already running")
	}

	var err error
	a.serial, err = transport.NewSerialTransport(port, baud)
	if err != nil {
		return err
	}

	a.golden = session.NewSession(port, baud, a.parser.Name())
	a.isRunning = true
	a.stopChan = make(chan struct{})

	go a.recordLoop()

	return nil
}

// recordLoop 录制循环
func (a *App) recordLoop() {
	buf := make([]byte, 4096)
	accumulated := make([]byte, 0, 8192)

	for {
		select {
		case <-a.stopChan:
			return
		default:
			n, err := a.serial.Read(buf)
			if err != nil {
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				// 添加到累积缓冲区
				accumulated = append(accumulated, data...)

				// 解析完整帧
				commands := a.parser.Parse(accumulated)

				// 清除已解析的部分
				if len(commands) > 0 {
					// 找到最后一个完整帧的结束位置
					// 简化处理：清除累积缓冲区
					accumulated = accumulated[:0]
				}

				// 发送命令到前端
				for _, cmd := range commands {
					record := session.Record{
						Index:     cmd.Index,
						Direction: cmd.Direction.String(),
						Name:      cmd.Name,
						RawData:   cmd.RawData,
						Timestamp: time.Now(),
					}
					a.golden.AddCommand(cmd)

					// 发送事件到前端
					runtime.EventsEmit(a.ctx, "record", map[string]interface{}{
						"index":     record.Index,
						"direction": record.Direction,
						"name":      record.Name,
						"raw_data":  hex.EncodeToString(record.RawData),
					})
				}
			}
		}
	}
}

// StopRecording 停止录制
func (a *App) StopRecording() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return nil
	}

	close(a.stopChan)
	a.isRunning = false

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

// GetRecords 获取记录列表
func (a *App) GetRecords() []map[string]interface{} {
	if a.golden == nil {
		return nil
	}

	records := make([]map[string]interface{}, len(a.golden.Records))
	for i, r := range a.golden.Records {
		records[i] = map[string]interface{}{
			"index":     r.Index,
			"direction": r.Direction,
			"name":      r.Name,
			"raw_data":  hex.EncodeToString(r.RawData),
		}
	}
	return records
}

// StartCompare 开始对比
func (a *App) StartCompare() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return "", fmt.Errorf("already running")
	}

	if a.golden == nil || len(a.golden.Records) == 0 {
		return "", fmt.Errorf("no golden session loaded, please record first")
	}

	var err error
	a.pty, err = transport.NewPtyTransport()
	if err != nil {
		return "", err
	}

	a.comparator = comparator.NewComparator(a.golden)
	a.compareIndex = 0
	a.isRunning = true
	a.stopChan = make(chan struct{})

	go a.compareLoop()

	return a.pty.SlavePath(), nil
}

// compareLoop 对比循环
func (a *App) compareLoop() {
	buf := make([]byte, 4096)
	accumulated := make([]byte, 0, 8192)

	for {
		select {
		case <-a.stopChan:
			return
		default:
			n, err := a.pty.Read(buf)
			if err != nil {
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				// 累积数据
				accumulated = append(accumulated, data...)

				// 解析帧
				commands := a.parser.Parse(accumulated)

				if len(commands) > 0 {
					accumulated = accumulated[:0]
				}

				// 对比每个命令
				for _, cmd := range commands {
					a.compareCommand(cmd)
				}
			}
		}
	}
}

// compareCommand 对比单个命令
func (a *App) compareCommand(cmd *protocol.Command) {
	if a.comparator == nil {
		return
	}

	// 获取基准记录
	var baseline *session.Record
	if a.compareIndex < len(a.golden.Records) {
		baseline = &a.golden.Records[a.compareIndex]
	}

	// 创建对比记录
	compare := &session.Record{
		Index:     cmd.Index,
		Direction: cmd.Direction.String(),
		Name:      cmd.Name,
		RawData:   cmd.RawData,
	}

	// 执行对比
	result := a.comparator.Compare(compare)
	a.compareIndex++

	// 发送事件到前端
	runtime.EventsEmit(a.ctx, "compare", map[string]interface{}{
		"index":     result.Index,
		"baseline":  formatRecord(baseline),
		"compare":   formatRecord(compare),
		"match":     result.Result == comparator.Match,
		"message":   result.Message,
	})

	// 更新统计
	matched, diff, _ := a.comparator.Stats()
	runtime.EventsEmit(a.ctx, "stats", map[string]int{
		"matched": matched,
		"diff":    diff,
	})
}

// formatRecord 格式化记录
func formatRecord(r *session.Record) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"index":     r.Index,
		"direction": r.Direction,
		"name":      r.Name,
		"raw_data":  hex.EncodeToString(r.RawData),
	}
}

// StopCompare 停止对比
func (a *App) StopCompare() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return nil
	}

	close(a.stopChan)
	a.isRunning = false

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