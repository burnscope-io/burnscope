//go:build darwin

package transport

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// PtyTransport PTY 虚拟串口传输
type PtyTransport struct {
	master     *os.File
	slavePath  string
	baudRate   int
	baudMu     sync.RWMutex
	baudChange chan int
}

// NewPtyTransport 创建 PTY 虚拟串口
func NewPtyTransport() (*PtyTransport, error) {
	// 打开 /dev/ptmx
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open ptmx: %w", err)
	}

	// 获取 slave 名称
	slavePath, err := getSlaveName(master)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("failed to get slave name: %w", err)
	}

	// 解锁 slave (TIOCPTYUNLK on macOS)
	unlock := 0
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(master.Fd()),
		0x20007452, // TIOCPTYUNLK on macOS
		uintptr(unsafe.Pointer(&unlock)),
	)
	if errno != 0 {
		master.Close()
		return nil, fmt.Errorf("failed to unlock pty: %v", errno)
	}

	t := &PtyTransport{
		master:     master,
		slavePath:  slavePath,
		baudRate:   0,
		baudChange: make(chan int, 10),
	}

	// 启动波特率监听
	go t.monitorBaudRate()

	return t, nil
}

// getSlaveName 获取 slave 设备名 (macOS)
func getSlaveName(master *os.File) (string, error) {
	var n [128]byte

	// TIOCPTYGNAME on macOS = 0x40807480
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(master.Fd()),
		0x40807480,
		uintptr(unsafe.Pointer(&n[0])),
	)
	if errno != 0 {
		return "", fmt.Errorf("TIOCPTYGNAME failed: %v", errno)
	}

	// 找到字符串结束
	var i int
	for i = 0; i < len(n) && n[i] != 0; i++ {
	}
	return string(n[:i]), nil
}

// termios 结构 (macOS)
type termios struct {
	Iflag  uint64
	Oflag  uint64
	Cflag  uint64
	Lflag  uint64
	Cc     [20]byte
	Ispeed uint64
	Ospeed uint64
}

// monitorBaudRate 监听波特率变化
func (t *PtyTransport) monitorBaudRate() {
	// 打开 slave 设备获取 termios
	slave, err := os.OpenFile(t.slavePath, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer slave.Close()

	var lastBaud int

	for {
		// 获取 termios
		var term termios
		_, _, errno := syscall.Syscall6(
			syscall.SYS_IOCTL,
			uintptr(slave.Fd()),
			0x40487413, // TCGETS on macOS
			uintptr(unsafe.Pointer(&term)),
			0, 0, 0,
		)

		if errno == 0 {
			baud := int(term.Ispeed)
			if baud > 0 && baud != lastBaud {
				lastBaud = baud
				t.baudMu.Lock()
				t.baudRate = baud
				t.baudMu.Unlock()
				select {
				case t.baudChange <- baud:
				default:
				}
			}
		}

		// 短间隔轮询
		syscall.Select(0, nil, nil, nil, &syscall.Timeval{Usec: 50000})
	}
}

// GetBaudRate 获取当前波特率
func (t *PtyTransport) GetBaudRate() int {
	t.baudMu.RLock()
	defer t.baudMu.RUnlock()
	return t.baudRate
}

// BaudChange 返回波特率变化通道
func (t *PtyTransport) BaudChange() <-chan int {
	return t.baudChange
}

// Read 从 master 读取数据
func (t *PtyTransport) Read(p []byte) (n int, err error) {
	return t.master.Read(p)
}

// Write 向 master 写入数据
func (t *PtyTransport) Write(p []byte) (n int, err error) {
	return t.master.Write(p)
}

// Close 关闭 PTY
func (t *PtyTransport) Close() error {
	if t.master != nil {
		return t.master.Close()
	}
	return nil
}

// Name 返回传输名称
func (t *PtyTransport) Name() string {
	return "pty"
}

// SlavePath 返回 slave 设备路径
func (t *PtyTransport) SlavePath() string {
	return t.slavePath
}