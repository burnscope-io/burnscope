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
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		writeData: make([]byte, 0),
	}
}

func (m *MockTransport) Read(p []byte) (n int, err error) {
	if m.readPos >= len(m.readData) {
		return 0, nil
	}
	n = copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *MockTransport) Write(p []byte) (n int, err error) {
	m.writeData = append(m.writeData, p...)
	return len(p), nil
}

func (m *MockTransport) Close() error {
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
}
