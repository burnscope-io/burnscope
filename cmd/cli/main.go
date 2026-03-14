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
	"github.com/burnscope-io/burnscope/internal/session"
	"github.com/burnscope-io/burnscope/internal/transport"
)

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
	flag.PrintDefaults()
}

// ==================== 录制模式 ====================

func runRecord(portName string, baud int, output string) {
	if portName == "" {
		fmt.Fprintln(os.Stderr, "错误: 需要指定 -port")
		os.Exit(1)
	}

	ports, _ := transport.ListPorts()
	fmt.Println("可用串口:")
	for _, p := range ports {
		marker := " "
		if p == portName {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, p)
	}

	fmt.Printf("\n连接 %s @ %d ...\n", portName, baud)
	serialPort, err := transport.NewSerialTransport(portName, baud)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开串口失败: %v\n", err)
		os.Exit(1)
	}
	defer serialPort.Close()

	sess := session.NewSession(portName, baud)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("录制中... (Ctrl+C 停止)")
	fmt.Println("─────────────────────────────────────────")

	buf := make([]byte, 4096)
	var mu sync.Mutex

	go func() {
		for {
			n, err := serialPort.Read(buf)
			if err != nil {
				continue
			}
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				mu.Lock()
				sess.Add(session.TX, data)
				mu.Unlock()

				fmt.Printf("[TX] %d bytes: %s\n", n, formatHex(data, 32))
			}
		}
	}()

	<-sigChan
	fmt.Println("\n─────────────────────────────────────────")

	mu.Lock()
	stats := sess.GetStats()
	if err := sess.Save(output); err != nil {
		fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
		os.Exit(1)
	}
	mu.Unlock()

	fmt.Printf("已保存: %s (%d 条记录)\n", output, stats.Total)
}

// ==================== 对比模式 ====================

func runCompare(input string) {
	if input == "" {
		fmt.Fprintln(os.Stderr, "错误: 需要指定 -input")
		os.Exit(1)
	}

	golden, err := session.Load(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("加载基准: %s (%d 条记录)\n", input, len(golden.Records))

	pty, err := transport.NewPtyTransport()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建虚拟串口失败: %v\n", err)
		os.Exit(1)
	}

	cmp := comparator.NewComparator(golden)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  虚拟串口: %s\n", pty.SlavePath())
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("请用烧录工具连接上述虚拟串口...")
	fmt.Println("按 Ctrl+C 结束")
	fmt.Println("─────────────────────────────────────────")

	buf := make([]byte, 4096)
	pos := 0
	var mu sync.Mutex

	for {
		select {
		case <-sigChan:
			fmt.Println("\n─────────────────────────────────────────")
			matched, diff, total := cmp.Stats()
			fmt.Printf("匹配: %d | 差异: %d | 总计: %d\n", matched, diff, total)
			return
		default:
			n, err := pty.Read(buf)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				mu.Lock()
				actual := &session.Record{
					Index:     pos + 1,
					Direction: session.TX,
					Data:      data,
				}
				result := cmp.Compare(actual)
				pos++
				mu.Unlock()

				// 打印对比结果
				if result.Expected != nil {
					fmt.Printf("基准: [%s] %s\n", result.Expected.Direction, formatHex(result.Expected.Data, 32))
				}
				fmt.Printf("对比: [TX] %s %s\n", formatHex(data, 32), result.Result)
				fmt.Println("─────────────────────────────────────────")

				// 返回响应
				if result.Expected != nil && pos < len(golden.Records) {
					next := golden.Records[pos]
					if next.Direction == session.RX {
						pty.Write(next.Data)
						pos++
					}
				}
			}
		}
	}
}

func formatHex(data []byte, maxLen int) string {
	h := hex.EncodeToString(data)
	if len(h) > maxLen {
		return h[:maxLen] + "..."
	}
	return h
}
