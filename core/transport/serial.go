package transport

import (
	"fmt"

	"go.bug.st/serial"
)

// SerialTransport 真实串口传输
type SerialTransport struct {
	port     serial.Port
	portName string
	baudRate int
}

// NewSerialTransport 创建串口传输
func NewSerialTransport(portName string, baudRate int) (*SerialTransport, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", portName, err)
	}

	return &SerialTransport{
		port:     port,
		portName: portName,
		baudRate: baudRate,
	}, nil
}

// Read 从串口读取数据
func (t *SerialTransport) Read(p []byte) (n int, err error) {
	return t.port.Read(p)
}

// Write 向串口写入数据
func (t *SerialTransport) Write(p []byte) (n int, err error) {
	return t.port.Write(p)
}

// Close 关闭串口
func (t *SerialTransport) Close() error {
	if t.port != nil {
		return t.port.Close()
	}
	return nil
}

// Name 返回传输名称
func (t *SerialTransport) Name() string {
	return "serial:" + t.portName
}

// SetDTR 设置 DTR 信号
func (t *SerialTransport) SetDTR(level bool) error {
	return t.port.SetDTR(level)
}

// SetRTS 设置 RTS 信号
func (t *SerialTransport) SetRTS(level bool) error {
	return t.port.SetRTS(level)
}

// GetBaudRate 获取波特率
func (t *SerialTransport) GetBaudRate() int {
	return t.baudRate
}

// GetPortName 获取端口名
func (t *SerialTransport) GetPortName() string {
	return t.portName
}

// ListPorts 列出可用串口
func ListPorts() ([]string, error) {
	return serial.GetPortsList()
}
