//go:build linux

package transport

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// PtyTransport PTY 虚拟串口传输
type PtyTransport struct {
	master    *os.File
	slavePath string
}

// NewPtyTransport 创建 PTY 虚拟串口
func NewPtyTransport() (*PtyTransport, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open ptmx: %w", err)
	}

	// 获取 slave 名称
	var ptn int32
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(master.Fd()),
		syscall.TIOCGPTN,
		uintptr(unsafe.Pointer(&ptn)),
	)
	if errno != 0 {
		master.Close()
		return nil, fmt.Errorf("TIOCGPTN failed: %v", errno)
	}
	slavePath := fmt.Sprintf("/dev/pts/%d", ptn)

	// 解锁 slave
	unlock := 0
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(master.Fd()),
		syscall.TIOCSPTLCK,
		uintptr(unsafe.Pointer(&unlock)),
	)
	if errno != 0 {
		master.Close()
		return nil, fmt.Errorf("TIOCSPTLCK failed: %v", errno)
	}

	return &PtyTransport{
		master:    master,
		slavePath: slavePath,
	}, nil
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

// GetBaudRate 获取当前波特率 (Linux PTY 不支持)
func (t *PtyTransport) GetBaudRate() int {
	return 0
}

// BaudChange 返回波特率变化通道 (Linux PTY 不支持)
func (t *PtyTransport) BaudChange() <-chan int {
	return nil
}
