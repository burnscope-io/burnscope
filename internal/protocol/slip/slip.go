// Package slip 实现 SLIP 编解码
// SLIP (Serial Line IP) 用于 ESP 烧录协议的数据帧封装
package slip

import "bytes"

const (
	End     = 0xC0 // 帧结束符
	Esc     = 0xDB // 转义符
	EscEnd  = 0xDC // 转义后的 End
	EscEsc  = 0xDD // 转义后的 Esc
)

// Encode 编码数据为 SLIP 帧
func Encode(data []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(End) // 帧开始

	for _, b := range data {
		switch b {
		case End:
			buf.WriteByte(Esc)
			buf.WriteByte(EscEnd)
		case Esc:
			buf.WriteByte(Esc)
			buf.WriteByte(EscEsc)
		default:
			buf.WriteByte(b)
		}
	}

	buf.WriteByte(End) // 帧结束
	return buf.Bytes()
}

// Decode 解码 SLIP 帧
// 返回完整帧列表（可能多个帧）
func Decode(data []byte) [][]byte {
	var frames [][]byte
	var frame []byte
	escaped := false

	for _, b := range data {
		if escaped {
			escaped = false
			switch b {
			case EscEnd:
				frame = append(frame, End)
			case EscEsc:
				frame = append(frame, Esc)
			default:
				// 无效转义，忽略
			}
			continue
		}

		switch b {
		case End:
			if len(frame) > 0 {
				frames = append(frames, frame)
				frame = nil
			}
		case Esc:
			escaped = true
		default:
			frame = append(frame, b)
		}
	}

	return frames
}

// IsFrameStart 检查是否是帧开始
func IsFrameStart(data []byte, pos int) bool {
	return pos < len(data) && data[pos] == End
}

// FindFrameEnd 查找帧结束位置
func FindFrameEnd(data []byte, start int) int {
	for i := start + 1; i < len(data); i++ {
		if data[i] == End {
			return i
		}
	}
	return -1
}
