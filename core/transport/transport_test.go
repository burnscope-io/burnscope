package transport

import (
	"testing"
)

func TestSerialTransportName(t *testing.T) {
	// 不能在没有真实串口的情况下测试 NewSerialTransport
	// 只测试接口定义
	var _ Transport = (*SerialTransport)(nil)
}

func TestPtyTransportInterface(t *testing.T) {
	// 验证 PtyTransport 实现了 Transport 接口
	var _ Transport = (*PtyTransport)(nil)
}

func TestTransportInterface(t *testing.T) {
	// 验证接口定义
	var _ Transport = (*MockTransport)(nil)
}

// MockTransport 用于测试的模拟传输
type MockTransport struct {
	readData  []byte
	readPos   int
	writeData []byte
	closed    bool
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		writeData: make([]byte, 0),
	}
}

func (m *MockTransport) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, nil
	}
	if m.readPos >= len(m.readData) {
		return 0, nil
	}
	n = copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *MockTransport) Write(p []byte) (n int, err error) {
	if m.closed {
		return 0, nil
	}
	m.writeData = append(m.writeData, p...)
	return len(p), nil
}

func (m *MockTransport) Close() error {
	m.closed = true
	return nil
}

func (m *MockTransport) Name() string {
	return "mock"
}

func (m *MockTransport) SetReadData(data []byte) {
	m.readData = data
	m.readPos = 0
}

func (m *MockTransport) GetWriteData() []byte {
	return m.writeData
}

func (m *MockTransport) IsClosed() bool {
	return m.closed
}

func TestMockTransport(t *testing.T) {
	m := NewMockTransport()
	m.SetReadData([]byte{0x01, 0x02, 0x03})

	buf := make([]byte, 10)
	n, err := m.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 3 {
		t.Errorf("Read() n = %d, want 3", n)
	}

	n, err = m.Write([]byte{0x04, 0x05})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 2 {
		t.Errorf("Write() n = %d, want 2", n)
	}

	if string(m.GetWriteData()) != "\x04\x05" {
		t.Errorf("GetWriteData() = %v, want [0x04, 0x05]", m.GetWriteData())
	}

	if m.Name() != "mock" {
		t.Errorf("Name() = %s, want mock", m.Name())
	}

	if err := m.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !m.IsClosed() {
		t.Error("IsClosed() = false, want true")
	}
}

func TestMockTransport_MultipleReadWrite(t *testing.T) {
	m := NewMockTransport()

	// Write multiple times
	m.Write([]byte{0x01})
	m.Write([]byte{0x02})
	m.Write([]byte{0x03})

	if len(m.GetWriteData()) != 3 {
		t.Errorf("write data length = %d, want 3", len(m.GetWriteData()))
	}

	// Read multiple times
	m.SetReadData([]byte{0xAA, 0xBB, 0xCC, 0xDD})

	buf := make([]byte, 2)
	n, _ := m.Read(buf)
	if n != 2 || buf[0] != 0xAA || buf[1] != 0xBB {
		t.Errorf("first read = %v, want [0xAA, 0xBB]", buf[:n])
	}

	n, _ = m.Read(buf)
	if n != 2 || buf[0] != 0xCC || buf[1] != 0xDD {
		t.Errorf("second read = %v, want [0xCC, 0xDD]", buf[:n])
	}

	// Read after end
	n, _ = m.Read(buf)
	if n != 0 {
		t.Errorf("read after end = %d, want 0", n)
	}
}

// MockSerialTransport 模拟串口传输
type MockSerialTransport struct {
	MockTransport
	portName string
	baudRate int
	dtr      bool
	rts      bool
}

func NewMockSerialTransport(portName string, baudRate int) *MockSerialTransport {
	return &MockSerialTransport{
		MockTransport: MockTransport{},
		portName:      portName,
		baudRate:      baudRate,
	}
}

func (m *MockSerialTransport) SetDTR(level bool) error {
	m.dtr = level
	return nil
}

func (m *MockSerialTransport) SetRTS(level bool) error {
	m.rts = level
	return nil
}

func (m *MockSerialTransport) GetDTR() bool {
	return m.dtr
}

func (m *MockSerialTransport) GetRTS() bool {
	return m.rts
}

func (m *MockSerialTransport) GetBaudRate() int {
	return m.baudRate
}

func (m *MockSerialTransport) GetPortName() string {
	return m.portName
}

func (m *MockSerialTransport) Name() string {
	return "serial:" + m.portName
}

func TestMockSerialTransport(t *testing.T) {
	m := NewMockSerialTransport("/dev/ttyUSB0", 115200)

	if m.GetPortName() != "/dev/ttyUSB0" {
		t.Errorf("port name = %s, want /dev/ttyUSB0", m.GetPortName())
	}

	if m.GetBaudRate() != 115200 {
		t.Errorf("baud rate = %d, want 115200", m.GetBaudRate())
	}

	if m.Name() != "serial:/dev/ttyUSB0" {
		t.Errorf("name = %s, want serial:/dev/ttyUSB0", m.Name())
	}

	// Test DTR/RTS
	if err := m.SetDTR(true); err != nil {
		t.Errorf("SetDTR error = %v", err)
	}
	if !m.GetDTR() {
		t.Error("DTR = false, want true")
	}

	if err := m.SetRTS(true); err != nil {
		t.Errorf("SetRTS error = %v", err)
	}
	if !m.GetRTS() {
		t.Error("RTS = false, want true")
	}

	// Test read/write
	m.SetReadData([]byte{0xC0, 0x00})
	buf := make([]byte, 10)
	n, err := m.Read(buf)
	if err != nil || n != 2 {
		t.Errorf("Read() = %d, %v, want 2, nil", n, err)
	}

	m.Write([]byte{0x01, 0x02})
	if len(m.GetWriteData()) != 2 {
		t.Errorf("write data length = %d, want 2", len(m.GetWriteData()))
	}

	// Test close
	if err := m.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}
}

func TestMockSerialTransport_ControlSignal(t *testing.T) {
	m := NewMockSerialTransport("/dev/ttyUSB1", 921600)

	// Test interface compliance
	var _ ControlSignal = m

	// Test DTR toggle
	m.SetDTR(true)
	m.SetDTR(false)
	if m.GetDTR() {
		t.Error("DTR should be false")
	}

	// Test RTS toggle
	m.SetRTS(true)
	if !m.GetRTS() {
		t.Error("RTS should be true")
	}
	m.SetRTS(false)
	if m.GetRTS() {
		t.Error("RTS should be false")
	}
}

func TestListPorts(t *testing.T) {
	ports, err := ListPorts()
	// ListPorts should not error, but may return empty list
	if err != nil {
		t.Errorf("ListPorts() error = %v", err)
	}
	// Just check it returns something (even if empty is OK)
	t.Logf("Found %d ports", len(ports))
	for _, p := range ports {
		t.Logf("  Port: %s", p)
	}
}
