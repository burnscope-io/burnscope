// Package esp 实现 ESP 烧录协议解析
package esp

import (
	"fmt"

	"github.com/burnscope-io/burnscope/internal/protocol"
	"github.com/burnscope-io/burnscope/internal/protocol/slip"
)

// ESP 命令码
const (
	CmdSync         = 0x08
	CmdReadReg      = 0x0A
	CmdWriteReg     = 0x09
	CmdSpiAttach    = 0x0D
	CmdSpiSetParams = 0x0B
	CmdFlashBegin   = 0x02
	CmdFlashData    = 0x03
	CmdFlashEnd     = 0x04
	CmdMemBegin     = 0x05
	CmdMemEnd       = 0x06
	CmdMemData      = 0x07
	CmdSyncFrame    = 0x00
)

// 命令名称映射
var commandNames = map[byte]string{
	0x00: "SYNC_FRAME",
	0x08: "SYNC",
	0x09: "WRITE_REG",
	0x0A: "READ_REG",
	0x0B: "SPI_SET_PARAMS",
	0x0C: "SPI_READ",
	0x0D: "SPI_ATTACH",
	0x02: "FLASH_BEGIN",
	0x03: "FLASH_DATA",
	0x04: "FLASH_END",
	0x05: "MEM_BEGIN",
	0x06: "MEM_END",
	0x07: "MEM_DATA",
	0x0E: "CHANGE_BAUDRATE",
	0x10: "DETECT_FLASH",
	0x11: "ERASE_FLASH",
	0x12: "ERASE_REGION",
}

// ESPFlashProtocol ESP Flash 协议解析器
type ESPFlashProtocol struct {
	frameIndex int
}

// NewESPFlashProtocol 创建解析器
func NewESPFlashProtocol() *ESPFlashProtocol {
	return &ESPFlashProtocol{frameIndex: 0}
}

// Name 返回协议名称
func (p *ESPFlashProtocol) Name() string {
	return "ESP-FLASH"
}

// Parse 解析原始数据
func (p *ESPFlashProtocol) Parse(data []byte) []*protocol.Command {
	frames := slip.Decode(data)
	var commands []*protocol.Command

	for _, frame := range frames {
		cmd := p.parseFrame(frame)
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return commands
}

// parseFrame 解析单个帧
func (p *ESPFlashProtocol) parseFrame(frame []byte) *protocol.Command {
	if len(frame) < 1 {
		return nil
	}

	p.frameIndex++

	// 判断方向：0x00 开头是请求，0x01 开头是响应
	direction := protocol.TX
	if frame[0] == 0x01 {
		direction = protocol.RX
	}

	// 获取命令码
	var cmdCode byte
	var name string

	if len(frame) >= 2 {
		cmdCode = frame[1]
		name = getCommandName(cmdCode)
	} else {
		name = "UNKNOWN"
	}

	return &protocol.Command{
		Index:     p.frameIndex,
		Direction: direction,
		Name:      name,
		RawData:   frame,
		Parsed:    p.parsePayload(cmdCode, frame),
	}
}

// getCommandName 获取命令名称
func getCommandName(code byte) string {
	if name, ok := commandNames[code]; ok {
		return name
	}
	return fmt.Sprintf("CMD_0x%02X", code)
}

// parsePayload 解析载荷
func (p *ESPFlashProtocol) parsePayload(cmdCode byte, frame []byte) any {
	// 根据命令类型解析不同的载荷
	// 这里先返回 nil，后续可以扩展
	switch cmdCode {
	case CmdSync:
		return p.parseSyncPayload(frame)
	case CmdFlashData:
		return p.parseFlashDataPayload(frame)
	default:
		return nil
	}
}

// SyncPayload SYNC 命令载荷
type SyncPayload struct {
	Command    byte
	Data       []byte
	Checksum   uint16
}

func (p *ESPFlashProtocol) parseSyncPayload(frame []byte) *SyncPayload {
	if len(frame) < 10 {
		return nil
	}
	return &SyncPayload{
		Command:  frame[1],
		Data:     frame[2:8],
		Checksum: uint16(frame[8]) | uint16(frame[9])<<8,
	}
}

// FlashDataPayload FLASH_DATA 命令载荷
type FlashDataPayload struct {
	Size     uint32
	Sequence uint32
	Data     []byte
	Checksum uint32
}

func (p *ESPFlashProtocol) parseFlashDataPayload(frame []byte) *FlashDataPayload {
	if len(frame) < 17 {
		return nil
	}
	return &FlashDataPayload{
		Size:     uint32(frame[2]) | uint32(frame[3])<<8 | uint32(frame[4])<<16 | uint32(frame[5])<<24,
		Sequence: uint32(frame[6]) | uint32(frame[7])<<8 | uint32(frame[8])<<16 | uint32(frame[9])<<24,
		Data:     frame[16:], // 跳过头部
	}
}

// ResetIndex 重置序号
func (p *ESPFlashProtocol) ResetIndex() {
	p.frameIndex = 0
}
