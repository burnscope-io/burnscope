package slip

import (
	"bytes"
	"testing"
)

func TestEncodeSimple(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	encoded := Encode(data)

	// 应该是 End + data + End
	expected := []byte{End, 0x01, 0x02, 0x03, End}
	if !bytes.Equal(encoded, expected) {
		t.Errorf("Encode() = %v, want %v", encoded, expected)
	}
}

func TestEncodeWithSpecialBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "contains End",
			input:    []byte{0x01, End, 0x02},
			expected: []byte{End, 0x01, Esc, EscEnd, 0x02, End},
		},
		{
			name:     "contains Esc",
			input:    []byte{0x01, Esc, 0x02},
			expected: []byte{End, 0x01, Esc, EscEsc, 0x02, End},
		},
		{
			name:     "contains both",
			input:    []byte{End, Esc},
			expected: []byte{End, Esc, EscEnd, Esc, EscEsc, End},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Encode(tt.input)
			if !bytes.Equal(encoded, tt.expected) {
				t.Errorf("Encode() = %v, want %v", encoded, tt.expected)
			}
		})
	}
}

func TestDecodeSimple(t *testing.T) {
	data := []byte{End, 0x01, 0x02, 0x03, End}
	frames := Decode(data)

	if len(frames) != 1 {
		t.Fatalf("len(frames) = %d, want 1", len(frames))
	}

	expected := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(frames[0], expected) {
		t.Errorf("frames[0] = %v, want %v", frames[0], expected)
	}
}

func TestDecodeMultipleFrames(t *testing.T) {
	data := []byte{End, 0x01, End, End, 0x02, End}
	frames := Decode(data)

	if len(frames) != 2 {
		t.Fatalf("len(frames) = %d, want 2", len(frames))
	}

	if !bytes.Equal(frames[0], []byte{0x01}) {
		t.Errorf("frames[0] = %v, want [0x01]", frames[0])
	}
	if !bytes.Equal(frames[1], []byte{0x02}) {
		t.Errorf("frames[1] = %v, want [0x02]", frames[1])
	}
}

func TestDecodeWithEscapedBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected [][]byte
	}{
		{
			name:     "escaped End",
			input:    []byte{End, 0x01, Esc, EscEnd, 0x02, End},
			expected: [][]byte{{0x01, End, 0x02}},
		},
		{
			name:     "escaped Esc",
			input:    []byte{End, 0x01, Esc, EscEsc, 0x02, End},
			expected: [][]byte{{0x01, Esc, 0x02}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := Decode(tt.input)
			if len(frames) != len(tt.expected) {
				t.Fatalf("len(frames) = %d, want %d", len(frames), len(tt.expected))
			}
			for i := range frames {
				if !bytes.Equal(frames[i], tt.expected[i]) {
					t.Errorf("frames[%d] = %v, want %v", i, frames[i], tt.expected[i])
				}
			}
		})
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	testData := [][]byte{
		{0x00, 0x01, 0x02, 0x03},
		{End, Esc, End, Esc},
		{0xFF, 0xFE, 0xFD},
		make([]byte, 256), // 长数据
	}

	// 填充长数据
	for i := range testData[3] {
		testData[3][i] = byte(i)
	}

	for _, data := range testData {
		encoded := Encode(data)
		frames := Decode(encoded)

		if len(frames) != 1 {
			t.Errorf("len(frames) = %d, want 1 for data %v", len(frames), data[:min(10, len(data))])
			continue
		}

		if !bytes.Equal(frames[0], data) {
			t.Errorf("roundtrip failed: got %v, want %v", frames[0][:min(10, len(frames[0]))], data[:min(10, len(data))])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestFindFrameEnd(t *testing.T) {
	data := []byte{End, 0x01, 0x02, End, 0x03}

	idx := FindFrameEnd(data, 0)
	if idx != 3 {
		t.Errorf("FindFrameEnd() = %d, want 3", idx)
	}

	idx = FindFrameEnd(data, 4)
	if idx != -1 {
		t.Errorf("FindFrameEnd() = %d, want -1 (no end found)", idx)
	}
}

func TestIsFrameStart(t *testing.T) {
	data := []byte{End, 0x01}

	if !IsFrameStart(data, 0) {
		t.Error("IsFrameStart(0) = false, want true")
	}
	if IsFrameStart(data, 1) {
		t.Error("IsFrameStart(1) = true, want false")
	}
}
