//go:build darwin

package transport

/*
#cgo CFLAGS: -Wno-unused-result
#include <stdlib.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdio.h>
#include <string.h>
#include <termios.h>

int open_ptmx() {
    return open("/dev/ptmx", O_RDWR);
}

int open_slave(const char* path) {
    return open(path, O_RDWR);
}

int pty_grantpt(int fd) {
    return grantpt(fd);
}

int pty_unlockpt(int fd) {
    return unlockpt(fd);
}

const char* pty_ptsname(int fd) {
    return ptsname(fd);
}

// 设置 PTY 为原始模式（禁用 echo、行缓冲等）
int set_raw_mode(int fd) {
    struct termios t;
    if (tcgetattr(fd, &t) < 0) {
        return -1;
    }
    // 禁用所有处理
    t.c_iflag &= ~(ICRNL | IGNCR | INLCR | ISTRIP | IXON | IXOFF);
    t.c_oflag &= ~(OPOST | ONLCR | OCRNL);
    t.c_lflag &= ~(ICANON | ECHO | ECHOE | ECHOK | ECHONL | ISIG | IEXTEN);
    t.c_cflag |= CREAD | CLOCAL;
    // 设置超时和最小读取字节数
    t.c_cc[VMIN] = 1;
    t.c_cc[VTIME] = 0;
    return tcsetattr(fd, TCSANOW, &t);
}
*/
import "C"

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
	slaveFd    int
	slaveFile  *os.File
	slavePath  string
	baudRate   int
	baudMu     sync.RWMutex
	baudChange chan int
}

// NewPtyTransport 创建 PTY 虚拟串口
func NewPtyTransport() (*PtyTransport, error) {
	// 使用 Go 原生方式打开 PTY master（支持 SetReadDeadline）
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/ptmx failed: %w", err)
	}

	// grantpt
	if C.pty_grantpt(C.int(master.Fd())) != 0 {
		master.Close()
		return nil, fmt.Errorf("grantpt failed")
	}

	// unlockpt
	if C.pty_unlockpt(C.int(master.Fd())) != 0 {
		master.Close()
		return nil, fmt.Errorf("unlockpt failed")
	}

	// ptsname
	slaveName := C.pty_ptsname(C.int(master.Fd()))
	if slaveName == nil {
		master.Close()
		return nil, fmt.Errorf("ptsname failed")
	}
	slavePath := C.GoString(slaveName)

	// 打开 slave 并设置原始模式
	slavePathC := C.CString(slavePath)
	slaveFd := C.open_slave(slavePathC)
	C.free(unsafe.Pointer(slavePathC))
	
	if slaveFd < 0 {
		master.Close()
		return nil, fmt.Errorf("open slave failed")
	}
	
	// 设置原始模式（禁用 echo、行缓冲等）
	if C.set_raw_mode(slaveFd) < 0 {
		C.close(slaveFd)
		master.Close()
		return nil, fmt.Errorf("set raw mode failed")
	}
	
	// 保存 slave fd 用于波特率监听
	slaveFile := os.NewFile(uintptr(slaveFd), slavePath)

	t := &PtyTransport{
		master:     master,
		slaveFd:    int(slaveFd),
		slaveFile:  slaveFile,
		slavePath:  slavePath,
		baudRate:   0,
		baudChange: make(chan int, 10),
	}

	// 启动波特率监听（可选，忽略错误）
	go t.monitorBaudRate()

	return t, nil
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

// monitorBaudRate 监听波特率变化（可选功能）
func (t *PtyTransport) monitorBaudRate() {
	if t.slaveFile == nil {
		return
	}

	var lastBaud int

	for {
		// 获取 termios (TCGETS on macOS = 0x40487413)
		var term termios
		_, _, errno := syscall.Syscall6(
			syscall.SYS_IOCTL,
			t.slaveFile.Fd(),
			0x40487413,
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
		tv := syscall.Timeval{Usec: 50000}
		syscall.Select(0, nil, nil, nil, &tv)
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
	if t.slaveFd > 0 {
		C.close(C.int(t.slaveFd))
	}
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