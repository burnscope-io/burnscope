//go:build darwin

package transport

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// PtyTransport PTY 虚拟串口传输
type PtyTransport struct {
	master    *os.File
	slavePath string
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

	return &PtyTransport{
		master:    master,
		slavePath: slavePath,
	}, nil
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
