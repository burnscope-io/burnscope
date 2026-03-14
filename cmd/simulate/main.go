package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/burnscope-io/burnscope/internal/comparator"
	"github.com/burnscope-io/burnscope/internal/session"
)

// MockTransport 模拟传输
type MockTransport struct {
	readChan  chan []byte
	writeChan chan []byte
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		readChan:  make(chan []byte, 100),
		writeChan: make(chan []byte, 100),
	}
}

func (m *MockTransport) Read(p []byte) (n int, err error) {
	data := <-m.readChan
	copy(p, data)
	return len(data), nil
}

func (m *MockTransport) Write(p []byte) (n int, err error) {
	m.writeChan <- p
	return len(p), nil
}

func (m *MockTransport) Close() error {
	close(m.readChan)
	close(m.writeChan)
	return nil
}

func (m *MockTransport) Send(data []byte) {
	m.readChan <- data
}

func (m *MockTransport) Receive() []byte {
	return <-m.writeChan
}

// ==================== 模拟录制 ====================

func simulateRecording() *session.Session {
	fmt.Println("\n=== 模拟录制（中间人模式）===")

	sess := session.NewSession("/dev/ttyUSB0", 115200)
	var mu sync.Mutex

	// 模拟 TX (烧录工具发送)
	txCommands := [][]byte{
		{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}, // SYNC
		{0xC0, 0x0B, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},       // SPI_ATTACH
		{0xC0, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},             // FLASH_BEGIN
	}

	// 模拟 RX (设备响应)
	rxResponses := [][]byte{
		{0xC0, 0x01, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}, // SYNC_REPLY
		{0xC0, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},       // SPI_ATTACH_OK
		{0xC0, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},             // FLASH_BEGIN_OK
	}

	// 记录交互
	for i := 0; i < len(txCommands); i++ {
		mu.Lock()
		// TX
		sess.Add(session.TX, txCommands[i])
		fmt.Printf("[TX] #%d: %s\n", i+1, formatHex(txCommands[i], 32))

		// RX
		sess.Add(session.RX, rxResponses[i])
		fmt.Printf("[RX] #%d: %s\n", i+1, formatHex(rxResponses[i], 32))
		fmt.Println("─────────────────────────────────────────")
		mu.Unlock()
	}

	stats := sess.GetStats()
	fmt.Printf("录制完成: TX=%d, RX=%d, Total=%d\n", stats.TXCount, stats.RXCount, stats.Total)

	return sess
}

// ==================== 模拟对比 ====================

func simulateCompare(golden *session.Session) {
	fmt.Println("\n=== 模拟对比 ===")

	cmp := comparator.NewComparator(golden)

	// 模拟完全匹配的输入
	matchedInputs := [][]byte{
		{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},
		{0xC0, 0x0B, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},
		{0xC0, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},
	}

	for i, data := range matchedInputs {
		actual := &session.Record{
			Index:     i + 1,
			Direction: session.TX,
			Data:      data,
		}

		result := cmp.Compare(actual)

		// 打印对比结果
		if result.ExpectedTX != nil {
			fmt.Printf("基准: [TX] %s\n", formatHex(result.ExpectedTX.Data, 32))
		}
		fmt.Printf("对比: [TX] %s %s\n", formatHex(data, 32), result.Result)

		// 回放 RX
		if result.ExpectedRX != nil {
			fmt.Printf("回放: [RX] %s\n", formatHex(result.ExpectedRX.Data, 32))
		}
		fmt.Println("─────────────────────────────────────────")
	}

	matched, diff, total := cmp.Stats()
	fmt.Printf("对比完成: 匹配=%d, 差异=%d, 总计=%d\n", matched, diff, total)
}

// ==================== 模拟差异对比 ====================

func simulateDiffCompare(golden *session.Session) {
	fmt.Println("\n=== 模拟差异对比 ===")

	cmp := comparator.NewComparator(golden)

	// 模拟有差异的输入
	diffInputs := [][]byte{
		{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}, // 匹配
		{0xC0, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},       // 差异
		{0xC0, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},             // 差异（跳过了原来的 TX）
	}

	for i, data := range diffInputs {
		actual := &session.Record{
			Index:     i + 1,
			Direction: session.TX,
			Data:      data,
		}

		result := cmp.Compare(actual)

		if result.ExpectedTX != nil {
			fmt.Printf("基准: [TX] %s\n", formatHex(result.ExpectedTX.Data, 32))
		}
		fmt.Printf("对比: [TX] %s %s\n", formatHex(data, 32), result.Result)
		fmt.Println("─────────────────────────────────────────")
	}

	matched, diff, total := cmp.Stats()
	fmt.Printf("对比完成: 匹配=%d, 差异=%d, 总计=%d\n", matched, diff, total)
}

// ==================== 保存和加载测试 ====================

func testSaveLoad(sess *session.Session) {
	fmt.Println("\n=== 保存/加载测试 ===")

	path := "/tmp/burnscope-test.golden"

	// 保存
	err := sess.Save(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
		return
	}
	fmt.Printf("已保存: %s\n", path)

	// 加载
	loaded, err := session.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载失败: %v\n", err)
		return
	}

	// 验证
	if len(loaded.Records) != len(sess.Records) {
		fmt.Fprintf(os.Stderr, "记录数不匹配: %d vs %d\n", len(loaded.Records), len(sess.Records))
		return
	}

	for i := range sess.Records {
		if !bytes.Equal(sess.Records[i].Data, loaded.Records[i].Data) {
			fmt.Fprintf(os.Stderr, "记录 %d 数据不匹配\n", i)
			return
		}
	}

	fmt.Printf("加载验证成功: %d 条记录\n", len(loaded.Records))
}

func formatHex(data []byte, maxLen int) string {
	h := hex.EncodeToString(data)
	if len(h) > maxLen {
		return h[:maxLen] + "..."
	}
	return h
}

func main() {
	// 模拟录制
	golden := simulateRecording()

	// 保存/加载测试
	testSaveLoad(golden)

	// 重新加载用于对比
	golden, _ = session.Load("/tmp/burnscope-test.golden")

	// 模拟完全匹配的对比
	simulateCompare(golden)

	// 重新加载用于差异对比
	golden, _ = session.Load("/tmp/burnscope-test.golden")

	// 模拟有差异的对比
	simulateDiffCompare(golden)

	fmt.Println("\n=== 所有测试完成 ===")
}