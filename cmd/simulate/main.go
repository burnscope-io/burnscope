package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/burnscope-io/burnscope/internal/comparator"
	"github.com/burnscope-io/burnscope/internal/session"
)

// ==================== Mock Transport ====================

// MockTransport 双向模拟传输
type MockTransport struct {
	name      string
	readBuf   []byte
	readPos   int
	writeBuf  []byte
	writeMu   sync.Mutex
	readChan  chan []byte
	writeChan chan []byte
	closed    bool
}

func NewMockTransport(name string) *MockTransport {
	return &MockTransport{
		name:      name,
		readChan:  make(chan []byte, 100),
		writeChan: make(chan []byte, 100),
	}
}

func (m *MockTransport) Read(p []byte) (n int, err error) {
	select {
	case data := <-m.readChan:
		n = copy(p, data)
		return n, nil
	default:
		time.Sleep(10 * time.Millisecond)
		return 0, nil
	}
}

func (m *MockTransport) Write(p []byte) (n int, err error) {
	m.writeMu.Lock()
	m.writeBuf = append(m.writeBuf, p...)
	m.writeMu.Unlock()
	m.writeChan <- p
	return len(p), nil
}

func (m *MockTransport) Close() error {
	m.closed = true
	return nil
}

func (m *MockTransport) Name() string {
	return m.name
}

// Send 模拟发送数据到 Read
func (m *MockTransport) Send(data []byte) {
	m.readChan <- data
}

// Receive 接收 Write 的数据
func (m *MockTransport) Receive() []byte {
	select {
	case data := <-m.writeChan:
		return data
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// ==================== 中间人录制器 ====================

// ManInTheMiddle 中间人录制器
type ManInTheMiddle struct {
	pty    *MockTransport
	serial *MockTransport
	sess   *session.Session
	stop   chan struct{}
	wg     sync.WaitGroup
}

func NewManInTheMiddle() *ManInTheMiddle {
	return &ManInTheMiddle{
		pty:    NewMockTransport("pty"),
		serial: NewMockTransport("serial"),
		sess:   session.NewSession("/dev/ttyUSB0", 115200),
		stop:   make(chan struct{}),
	}
}

// Start 启动双向转发和录制
func (m *ManInTheMiddle) Start() {
	// PTY -> Serial (TX)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-m.stop:
				return
			default:
				n, err := m.pty.Read(buf)
				if err != nil || n == 0 {
					continue
				}
				data := make([]byte, n)
				copy(data, buf[:n])

				// 记录 TX
				m.sess.Add(session.TX, data)

				// 转发到串口
				m.serial.Write(data)
			}
		}
	}()

	// Serial -> PTY (RX)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-m.stop:
				return
			default:
				n, err := m.serial.Read(buf)
				if err != nil || n == 0 {
					continue
				}
				data := make([]byte, n)
				copy(data, buf[:n])

				// 记录 RX
				m.sess.Add(session.RX, data)

				// 转发到 PTY
				m.pty.Write(data)
			}
		}
	}()
}

// Stop 停止录制
func (m *ManInTheMiddle) Stop() {
	close(m.stop)
	m.wg.Wait()
}

// GetSession 获取会话
func (m *ManInTheMiddle) GetSession() *session.Session {
	return m.sess
}

// ==================== 中间人模式录制测试 ====================

func testManInTheMiddleRecording() {
	fmt.Println("\n=== 中间人模式录制测试 ===")

	m := NewManInTheMiddle()
	m.Start()

	// 模拟烧录工具发送的命令
	txCommands := [][]byte{
		{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}, // SYNC
		{0xC0, 0x0B, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},       // SPI_ATTACH
		{0xC0, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},             // FLASH_BEGIN
	}

	// 模拟设备返回的响应
	rxResponses := [][]byte{
		{0xC0, 0x01, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}, // SYNC_REPLY
		{0xC0, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},       // SPI_ATTACH_OK
		{0xC0, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},             // FLASH_BEGIN_OK
	}

	// 模拟交互
	for i := 0; i < len(txCommands); i++ {
		// 模拟烧录工具发送 TX
		m.pty.Send(txCommands[i])
		time.Sleep(50 * time.Millisecond)

		// 验证 TX 被转发到串口
		forwarded := m.serial.Receive()
		if !bytes.Equal(forwarded, txCommands[i]) {
			fmt.Printf("❌ TX #%d 转发失败\n", i+1)
		} else {
			fmt.Printf("[TX] #%d: %s\n", i+1, formatHex(txCommands[i], 32))
		}

		// 模拟设备返回 RX
		m.serial.Send(rxResponses[i])
		time.Sleep(50 * time.Millisecond)

		// 验证 RX 被转发回 PTY
		response := m.pty.Receive()
		if !bytes.Equal(response, rxResponses[i]) {
			fmt.Printf("❌ RX #%d 转发失败\n", i+1)
		} else {
			fmt.Printf("[RX] #%d: %s\n", i+1, formatHex(rxResponses[i], 32))
		}
		fmt.Println("─────────────────────────────────────────")
	}

	m.Stop()

	stats := m.GetSession().GetStats()
	fmt.Printf("录制完成: TX=%d, RX=%d, Total=%d\n", stats.TXCount, stats.RXCount, stats.Total)

	// 验证录制内容
	sess := m.GetSession()
	if len(sess.Records) != 6 {
		fmt.Printf("❌ 记录数应为 6，实际 %d\n", len(sess.Records))
		return
	}

	// 验证交替顺序
	for i := 0; i < 3; i++ {
		txIdx := i * 2
		rxIdx := i*2 + 1

		if sess.Records[txIdx].Direction != session.TX {
			fmt.Printf("❌ 记录 %d 应为 TX\n", txIdx)
		}
		if sess.Records[rxIdx].Direction != session.RX {
			fmt.Printf("❌ 记录 %d 应为 RX\n", rxIdx)
		}
		if !bytes.Equal(sess.Records[txIdx].Data, txCommands[i]) {
			fmt.Printf("❌ TX #%d 数据不匹配\n", i+1)
		}
		if !bytes.Equal(sess.Records[rxIdx].Data, rxResponses[i]) {
			fmt.Printf("❌ RX #%d 数据不匹配\n", i+1)
		}
	}

	fmt.Println("✅ 中间人录制验证通过")
}

// ==================== 对比测试 ====================

func testCompare(golden *session.Session, inputs [][]byte, expectMatch []bool) {
	cmp := comparator.NewComparator(golden)

	for i, data := range inputs {
		actual := &session.Record{
			Direction: session.TX,
			Data:      data,
		}
		result := cmp.Compare(actual)

		if result.ExpectedTX != nil {
			fmt.Printf("基准: [TX] %s\n", formatHex(result.ExpectedTX.Data, 32))
		}
		fmt.Printf("对比: [TX] %s %s\n", formatHex(data, 32), result.Result)

		if result.ExpectedRX != nil {
			fmt.Printf("回放: [RX] %s\n", formatHex(result.ExpectedRX.Data, 32))
		}
		fmt.Println("─────────────────────────────────────────")

		// 验证期望
		isMatch := result.Result == comparator.Match
		if isMatch != expectMatch[i] {
			fmt.Printf("❌ 结果不符预期: 期望 match=%v, 实际 match=%v\n", expectMatch[i], isMatch)
		}
	}

	matched, diff, total := cmp.Stats()
	fmt.Printf("对比完成: 匹配=%d, 差异=%d, 总计=%d\n", matched, diff, total)
}

// ==================== 保存加载测试 ====================

func testSaveLoad(sess *session.Session) {
	fmt.Println("\n=== 保存/加载测试 ===")

	path := "/tmp/burnscope-mitm-test.golden"

	err := sess.Save(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
		return
	}
	fmt.Printf("已保存: %s\n", path)

	loaded, err := session.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载失败: %v\n", err)
		return
	}

	if len(loaded.Records) != len(sess.Records) {
		fmt.Fprintf(os.Stderr, "记录数不匹配\n")
		return
	}

	for i := range sess.Records {
		if sess.Records[i].Direction != loaded.Records[i].Direction {
			fmt.Fprintf(os.Stderr, "记录 %d 方向不匹配\n", i)
			return
		}
		if !bytes.Equal(sess.Records[i].Data, loaded.Records[i].Data) {
			fmt.Fprintf(os.Stderr, "记录 %d 数据不匹配\n", i)
			return
		}
	}

	fmt.Println("✅ 保存/加载验证通过")
}

func formatHex(data []byte, maxLen int) string {
	h := hex.EncodeToString(data)
	if len(h) > maxLen {
		return h[:maxLen] + "..."
	}
	return h
}

func main() {
	// 1. 中间人模式录制测试
	testManInTheMiddleRecording()

	// 2. 保存加载测试
	sess := NewManInTheMiddle().sess
	// 手动添加测试数据
	sess.Add(session.TX, []byte{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0})
	sess.Add(session.RX, []byte{0xC0, 0x01, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0})
	sess.Add(session.TX, []byte{0xC0, 0x0B, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0})
	sess.Add(session.RX, []byte{0xC0, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0})
	testSaveLoad(sess)

	// 3. 完全匹配对比测试
	fmt.Println("\n=== 完全匹配对比 ===")
	golden, _ := session.Load("/tmp/burnscope-mitm-test.golden")
	testCompare(golden, [][]byte{
		{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},
		{0xC0, 0x0B, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},
	}, []bool{true, true})

	// 4. 差异对比测试
	fmt.Println("\n=== 差异对比 ===")
	golden, _ = session.Load("/tmp/burnscope-mitm-test.golden")
	testCompare(golden, [][]byte{
		{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}, // 匹配
		{0xC0, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},       // 差异
	}, []bool{true, false})

	fmt.Println("\n=== 所有测试完成 ===")
}
