package esp

import (
	"testing"

	"github.com/burnscope-io/burnscope/internal/protocol"
)

func TestESPFlashProtocolName(t *testing.T) {
	p := NewESPFlashProtocol()
	if p.Name() != "ESP-FLASH" {
		t.Errorf("Name() = %s, want ESP-FLASH", p.Name())
	}
}

func TestParseSyncFrame(t *testing.T) {
	p := NewESPFlashProtocol()

	// SYNC 命令帧：C0 00 08 24 00 00 00 00 00 00 00 00 00 00 00 00 C0
	// 解码后：00 08 24 00 00 00 00 00 00 00 00 00 00 00 00
	data := []byte{0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}

	commands := p.Parse(data)
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}

	cmd := commands[0]
	if cmd.Direction != protocol.TX {
		t.Errorf("cmd.Direction = %v, want TX", cmd.Direction)
	}
	if cmd.Name != "SYNC" {
		t.Errorf("cmd.Name = %s, want SYNC", cmd.Name)
	}
}

func TestParseResponseFrame(t *testing.T) {
	p := NewESPFlashProtocol()

	// 响应帧：C0 01 08 00 00 00 00 00 C0
	// 解码后：01 08 00 00 00 00 00
	data := []byte{0xC0, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}

	commands := p.Parse(data)
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}

	cmd := commands[0]
	if cmd.Direction != protocol.RX {
		t.Errorf("cmd.Direction = %v, want RX", cmd.Direction)
	}
}

func TestParseMultipleFrames(t *testing.T) {
	p := NewESPFlashProtocol()

	// 两个帧
	data := []byte{
		0xC0, 0x00, 0x08, 0x24, 0x00, 0x00, 0xC0, // SYNC
		0xC0, 0x01, 0x08, 0x00, 0x00, 0x00, 0xC0, // SYNC response
	}

	commands := p.Parse(data)
	if len(commands) != 2 {
		t.Fatalf("len(commands) = %d, want 2", len(commands))
	}

	if commands[0].Direction != protocol.TX {
		t.Errorf("commands[0].Direction = %v, want TX", commands[0].Direction)
	}
	if commands[1].Direction != protocol.RX {
		t.Errorf("commands[1].Direction = %v, want RX", commands[1].Direction)
	}
}

func TestCommandNames(t *testing.T) {
	tests := []struct {
		code     byte
		expected string
	}{
		{CmdSync, "SYNC"},
		{CmdSpiAttach, "SPI_ATTACH"},
		{CmdFlashBegin, "FLASH_BEGIN"},
		{CmdFlashData, "FLASH_DATA"},
		{CmdFlashEnd, "FLASH_END"},
		{0xFF, "CMD_0xFF"}, // 未知命令
	}

	for _, tt := range tests {
		name := getCommandName(tt.code)
		if name != tt.expected {
			t.Errorf("getCommandName(0x%02X) = %s, want %s", tt.code, name, tt.expected)
		}
	}
}

func TestResetIndex(t *testing.T) {
	p := NewESPFlashProtocol()

	data := []byte{0xC0, 0x00, 0x08, 0xC0}
	p.Parse(data) // index -> 1
	p.Parse(data) // index -> 2

	commands := p.Parse(data) // index -> 3
	if commands[0].Index != 3 {
		t.Errorf("commands[0].Index = %d, want 3", commands[0].Index)
	}

	p.ResetIndex()
	commands = p.Parse(data)
	if commands[0].Index != 1 {
		t.Errorf("commands[0].Index = %d, want 1 after reset", commands[0].Index)
	}
}

func TestParseEmptyData(t *testing.T) {
	p := NewESPFlashProtocol()
	commands := p.Parse([]byte{})

	if len(commands) != 0 {
		t.Errorf("len(commands) = %d, want 0 for empty data", len(commands))
	}
}
