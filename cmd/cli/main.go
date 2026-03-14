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
	port := flag.String("port", "", "真实设备串口 (录制模式)")
	baud := flag.Int("baud", 115200, "波特率")
	output := flag.String("output", "session.golden", "输出文件")
	input := flag.String("input", "", "基准文件 (对比模式)")
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
	fmt.Println("burnscope - 烧录工具一致性验证 (中间人模式)")
	fmt.Println()
	fmt.Println("录制模式:")
	fmt.Println("  burnscope -mode record -port /dev/ttyUSB0 -output session.golden")
	fmt.Println("  输出虚拟串口路径，用 idf.py 连接该虚拟串口")
	fmt.Println()
	fmt.Println("对比模式:")
	fmt.Println("  burnscope -mode compare -input session.golden")
	fmt.Println("  输出虚拟串口路径，用 zyrthi-flash 连接该虚拟串口")
	fmt.Println()
	flag.PrintDefaults()
}

// ==================== 录制模式（中间人） ====================

func runRecord(devicePort string, baud int, output string) {
	if devicePort == "" {
		fmt.Fprintln(os.Stderr, "错误: 需要指定 -port (真实设备串口)")
		os.Exit(1)
	}

	// 列出可用串口
	ports, _ := transport.ListPorts()
	fmt.Println("可用串口:")
	for _, p := range ports {
		marker := " "
		if p == devicePort {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, p)
	}

	// 创建 PTY 虚拟串口
	pty, err := transport.NewPtyTransport()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建虚拟串口失败: %v\n", err)
		os.Exit(1)
	}

	// 连接真实设备
	serial, err := transport.NewSerialTransport(devicePort, baud)
	if err != nil {
		pty.Close()
		fmt.Fprintf(os.Stderr, "连接设备失败: %v\n", err)
		os.Exit(1)
	}

	sess := session.NewSession(devicePort, baud)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  中间人模式：录制 TX/RX")
	fmt.Printf("  虚拟串口: %s\n", pty.SlavePath())
	fmt.Printf("  真实设备: %s @ %d\n", devicePort, baud)
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("请用 idf.py 连接上述虚拟串口进行烧录...")
	fmt.Println("按 Ctrl+C 停止录制")
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")

	var mu sync.Mutex
	stopChan := make(chan struct{})

	// 烧录工具 → 设备 (TX)
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-stopChan:
				return
			default:
				n, err := pty.Read(buf)
				if err != nil {
					continue
				}
				if n > 0 {
					// 转发到设备
					serial.Write(buf[:n])

					// 记录 TX
					data := make([]byte, n)
					copy(data, buf[:n])
					mu.Lock()
					sess.Add(session.TX, data)
					mu.Unlock()

					fmt.Printf("[TX] %d: %s\n", n, formatHex(data, 32))
				}
			}
		}
	}()

	// 设备 → 烧录工具 (RX)
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-stopChan:
				return
			default:
				n, err := serial.Read(buf)
				if err != nil {
					continue
				}
				if n > 0 {
					// 转发到烧录工具
					pty.Write(buf[:n])

					// 记录 RX
					data := make([]byte, n)
					copy(data, buf[:n])
					mu.Lock()
					sess.Add(session.RX, data)
					mu.Unlock()

					fmt.Printf("[RX] %d: %s\n", n, formatHex(data, 32))
				}
			}
		}
	}()

	// 等待退出
	<-sigChan
	close(stopChan)

	fmt.Println("\n─────────────────────────────────────────")
	fmt.Println("停止录制...")

	pty.Close()
	serial.Close()

	mu.Lock()
	stats := sess.GetStats()
	err = sess.Save(output)
	mu.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已保存: %s\n", output)
	fmt.Printf("记录数: %d (TX: %d, RX: %d)\n", stats.Total, stats.TXCount, stats.RXCount)
}

// ==================== 对比模式 ====================

func runCompare(input string) {
	if input == "" {
		fmt.Fprintln(os.Stderr, "错误: 需要指定 -input")
		os.Exit(1)
	}

	golden, err := session.Load(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载基准失败: %v\n", err)
		os.Exit(1)
	}

	pty, err := transport.NewPtyTransport()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建虚拟串口失败: %v\n", err)
		os.Exit(1)
	}

	cmp := comparator.NewComparator(golden)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	stats := golden.GetStats()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  对比模式")
	fmt.Printf("  虚拟串口: %s\n", pty.SlavePath())
	fmt.Printf("  基准记录: %d 条 (TX: %d, RX: %d)\n", stats.Total, stats.TXCount, stats.RXCount)
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("请用 zyrthi-flash 连接上述虚拟串口进行烧录...")
	fmt.Println("按 Ctrl+C 结束")
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")

	buf := make([]byte, 4096)
	pos := 0
	var mu sync.Mutex
	stopChan := make(chan struct{})

	for {
		select {
		case <-sigChan:
			close(stopChan)
			fmt.Println("\n─────────────────────────────────────────")
			matched, diff, total := cmp.Stats()
			fmt.Printf("匹配: %d | 差异: %d | 总计: %d\n", matched, diff, total)
			pty.Close()
			return

		case <-stopChan:
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
						fmt.Printf("[回放] [RX] %s\n", formatHex(next.Data, 32))
						fmt.Println("─────────────────────────────────────────")
						pos++
					}
				}
				mu.Unlock()
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