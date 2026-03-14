//go:build !wails

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/burnscope-io/burnscope/internal/comparator"
	"github.com/burnscope-io/burnscope/internal/protocol"
	"github.com/burnscope-io/burnscope/internal/protocol/esp"
	"github.com/burnscope-io/burnscope/internal/session"
	"github.com/burnscope-io/burnscope/internal/transport"
)

// CLI 入口（非 Wails 构建）

func main() {
	mode := flag.String("mode", "", "模式: record 或 compare")
	port := flag.String("port", "", "串口设备")
	baud := flag.Int("baud", 115200, "波特率")
	output := flag.String("output", "session.golden", "输出文件")
	input := flag.String("input", "", "输入文件（对比模式）")
	flag.Parse()

	if *mode == "" {
		printUsage()
		os.Exit(1)
	}

	switch *mode {
	case "record":
		runRecord(*port, *baud, *output)
	case "compare":
		runCompare(*input)
	default:
		fmt.Fprintf(os.Stderr, "未知模式: %s\n", *mode)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("burnscope - 烧录工具一致性验证")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  burnscope -mode record -port /dev/ttyUSB0 -output session.golden")
	fmt.Println("  burnscope -mode compare -input session.golden")
	fmt.Println()
	fmt.Println("模式:")
	fmt.Println("  record   录制模式：连接真实设备，记录交互")
	fmt.Println("  compare  对比模式：创建虚拟串口，回放对比")
	fmt.Println()
	fmt.Println("选项:")
	flag.PrintDefaults()
}

// ==================== 录制模式 ====================

func runRecord(portName string, baud int, output string) {
	if portName == "" {
		fmt.Fprintln(os.Stderr, "错误: 录制模式需要指定 -port")
		os.Exit(1)
	}

	// 列出可用串口
	ports, err := transport.ListPorts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取串口列表失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("可用串口:")
	for _, p := range ports {
		marker := " "
		if p == portName {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, p)
	}

	// 打开串口
	fmt.Printf("\n连接 %s @ %d ...\n", portName, baud)
	serialPort, err := transport.NewSerialTransport(portName, baud)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开串口失败: %v\n", err)
		os.Exit(1)
	}
	defer serialPort.Close()

	// 创建会话
	sess := session.NewSession(portName, baud, "ESP-FLASH")
	parser := esp.NewESPFlashProtocol()

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("录制中... (Ctrl+C 停止)")
	fmt.Println("─────────────────────────────────────────────────")

	buf := make([]byte, 4096)
	recordChan := make(chan []byte, 100)

	// 读取协程
	go func() {
		for {
			n, err := serialPort.Read(buf)
			if err != nil {
				close(recordChan)
				return
			}
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				recordChan <- data
			}
		}
	}()

	// 主循环
	for {
		select {
		case <-sigChan:
			fmt.Println("\n─────────────────────────────────────────────────")
			fmt.Println("停止录制...")

			// 保存会话
			if err := sess.Save(output); err != nil {
				fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
				os.Exit(1)
			}

			stats := sess.GetStats()
			fmt.Printf("已保存到: %s\n", output)
			fmt.Printf("记录数: %d (TX: %d, RX: %d)\n", stats.TotalCommands, stats.TXCount, stats.RXCount)
			return

		case data, ok := <-recordChan:
			if !ok {
				fmt.Println("\n串口已关闭")
				return
			}

			// 解析命令
			commands := parser.Parse(data)
			for _, cmd := range commands {
				sess.AddCommand(cmd)
				printCommand(cmd, "基准")
			}
		}
	}
}

// ==================== 对比模式 ====================

type CompareSession struct {
	golden     *session.Session
	comparator *comparator.Comparator
	parser     protocol.Parser
	pty        *transport.PtyTransport

	mu       sync.Mutex
	position int
	stopChan chan struct{}
}

func runCompare(input string) {
	if input == "" {
		fmt.Fprintln(os.Stderr, "错误: 对比模式需要指定 -input")
		os.Exit(1)
	}

	// 加载黄金记录
	golden, err := session.Load(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载黄金记录失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("加载黄金记录: %s\n", input)
	fmt.Printf("记录数: %d\n\n", len(golden.Records))

	// 创建 PTY
	pty, err := transport.NewPtyTransport()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建虚拟串口失败: %v\n", err)
		fmt.Fprintln(os.Stderr, "\n提示: 请确保在真实终端环境中运行（非沙箱）")
		os.Exit(1)
	}

	// 创建对比会话
	cs := &CompareSession{
		golden:     golden,
		comparator: comparator.NewComparator(golden),
		parser:     esp.NewESPFlashProtocol(),
		pty:        pty,
		stopChan:   make(chan struct{}),
	}

	defer cs.Stop()

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  虚拟串口: %s\n", pty.SlavePath())
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("请用烧录工具连接上述虚拟串口进行烧录...")
	fmt.Println("按 Ctrl+C 结束")
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────")

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动读取协程
	go cs.readLoop()

	// 等待退出
	<-sigChan
	fmt.Println("\n─────────────────────────────────────────────────")
	fmt.Println("结束对比")

	// 打印统计
	matched, diff, total := cs.comparator.Stats()
	fmt.Printf("匹配: %d | 差异: %d | 总计: %d\n", matched, diff, total)
}

func (cs *CompareSession) readLoop() {
	buf := make([]byte, 4096)
	accumulated := make([]byte, 0, 8192)

	for {
		select {
		case <-cs.stopChan:
			return
		default:
			n, err := cs.pty.Read(buf)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				accumulated = append(accumulated, data...)

				// 解析帧
				commands := cs.parser.Parse(accumulated)

				if len(commands) > 0 {
					// 清除已解析的数据
					accumulated = accumulated[:0]
				}

				// 处理每个命令
				for _, cmd := range commands {
					cs.handleCommand(cmd)
				}
			}
		}
	}
}

func (cs *CompareSession) handleCommand(cmd *protocol.Command) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 创建对比记录
	compareRecord := &session.Record{
		Index:     cmd.Index,
		Direction: cmd.Direction.String(),
		Name:      cmd.Name,
		RawData:   cmd.RawData,
	}

	// 执行对比
	result := cs.comparator.Compare(compareRecord)

	// 获取基准记录
	var baseline *session.Record
	if result.Index > 0 && result.Index <= len(cs.golden.Records) {
		baseline = &cs.golden.Records[result.Index-1]
	}

	// 打印对比结果
	printCompareLine(baseline, compareRecord, result.Result)

	// 如果是 TX 命令，等待并返回对应的 RX 响应
	if cmd.Direction == protocol.TX {
		// 查找下一个 RX 记录
		pos, _ := cs.comparator.Progress()
		for i := pos; i < len(cs.golden.Records); i++ {
			record := cs.golden.Records[i]
			if record.Direction == "RX" {
				// 发送响应
				cs.pty.Write(record.RawData)
				printCommandFromRecord(record, "回放")
				break
			}
		}
	}
}

func (cs *CompareSession) Stop() {
	close(cs.stopChan)
	if cs.pty != nil {
		cs.pty.Close()
	}
}

// ==================== 打印辅助函数 ====================

func printCommand(cmd *protocol.Command, label string) {
	dir := "TX"
	if cmd.Direction == protocol.RX {
		dir = "RX"
	}
	dataStr := formatHex(cmd.RawData, 24)
	fmt.Printf("%s: %s %-12s %s\n", label, dir, cmd.Name, dataStr)
}

func printCommandFromRecord(r session.Record, label string) {
	dataStr := formatHex(r.RawData, 24)
	fmt.Printf("%s: %s %-12s %s\n", label, r.Direction, r.Name, dataStr)
}

func printCompareLine(baseline *session.Record, compare *session.Record, result comparator.Result) {
	// 打印基准行
	if baseline != nil {
		fmt.Printf("基准: %s %-12s %s\n", baseline.Direction, baseline.Name, formatHex(baseline.RawData, 24))
	} else {
		fmt.Println("基准: (无)")
	}

	// 打印对比行
	resultStr := result.String()
	fmt.Printf("对比: %s %-12s %s %s\n", compare.Direction, compare.Name, formatHex(compare.RawData, 24), resultStr)

	// 分隔线
	fmt.Println("─────────────────────────────────────────────────")
}

func formatHex(data []byte, maxLen int) string {
	if len(data) == 0 {
		return "(空)"
	}
	hexStr := hex.EncodeToString(data)
	if len(hexStr) > maxLen*2 {
		return hexStr[:maxLen*2] + "..."
	}
	return hexStr
}