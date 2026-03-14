package protocol

import "testing"

func TestDirectionString(t *testing.T) {
	tests := []struct {
		dir      Direction
		expected string
	}{
		{TX, "TX"},
		{RX, "RX"},
	}

	for _, tt := range tests {
		if got := tt.dir.String(); got != tt.expected {
			t.Errorf("Direction.String() = %s, want %s", got, tt.expected)
		}
	}
}

func TestCommandFields(t *testing.T) {
	cmd := &Command{
		Index:     1,
		Direction: TX,
		Name:      "SYNC",
		RawData:   []byte{0xc0, 0x00, 0x08},
	}

	if cmd.Index != 1 {
		t.Errorf("cmd.Index = %d, want 1", cmd.Index)
	}
	if cmd.Direction != TX {
		t.Errorf("cmd.Direction = %v, want TX", cmd.Direction)
	}
	if cmd.Name != "SYNC" {
		t.Errorf("cmd.Name = %s, want SYNC", cmd.Name)
	}
	if len(cmd.RawData) != 3 {
		t.Errorf("len(cmd.RawData) = %d, want 3", len(cmd.RawData))
	}
}
