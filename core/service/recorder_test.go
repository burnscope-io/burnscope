package service

import (
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/burnscope-io/burnscope/core/api"
	"github.com/burnscope-io/burnscope/core/session"
)

func TestNewService(t *testing.T) {
	s := NewService()

	if s.mode != api.ModeIdle {
		t.Errorf("expected mode to be idle, got %s", s.mode)
	}

	if s.upperPort != "" {
		t.Errorf("expected empty upperPort, got %s", s.upperPort)
	}

	if len(s.lowerPorts) != 0 {
		t.Errorf("expected empty lowerPorts, got %d", len(s.lowerPorts))
	}

	if len(s.baseline) != 0 {
		t.Errorf("expected empty baseline, got %d", len(s.baseline))
	}

	if len(s.actual) != 0 {
		t.Errorf("expected empty actual, got %d", len(s.actual))
	}
}

func TestGetState(t *testing.T) {
	s := NewService()
	state := s.GetState()

	if state.Mode != string(api.ModeIdle) {
		t.Errorf("expected mode idle, got %s", state.Mode)
	}

	if state.UpperPort != "" {
		t.Errorf("expected empty upperPort, got %s", state.UpperPort)
	}

	if len(state.LowerPorts) != 0 {
		t.Errorf("expected empty lowerPorts, got %d", len(state.LowerPorts))
	}
}

func TestInit(t *testing.T) {
	s := NewService()
	state, err := s.Init()

	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if state.UpperPort == "" {
		t.Error("expected upperPort to be set")
	}

	if len(state.LowerPorts) == 0 {
		t.Error("expected at least one lower port (virtual)")
	}

	// Check virtual port exists
	found := false
	for _, p := range state.LowerPorts {
		if p.PortType == "virtual" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected virtual port in lowerPorts")
	}

	// Cleanup
	s.Stop()
}

func TestRefreshPorts(t *testing.T) {
	s := NewService()

	// Init first
	_, err := s.Init()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	state := s.RefreshPorts()

	if len(state.LowerPorts) == 0 {
		t.Error("expected at least one lower port")
	}

	// Cleanup
	s.Stop()
}

func TestStartRecord(t *testing.T) {
	s := NewService()

	state, err := s.StartRecord()
	if err != nil {
		t.Fatalf("startRecord failed: %v", err)
	}

	if state.Mode != string(api.ModeRecord) {
		t.Errorf("expected mode record, got %s", state.Mode)
	}

	// Cleanup
	s.Stop()
}

func TestStartRecord_Idempotent(t *testing.T) {
	s := NewService()

	// Start record
	s.StartRecord()
	state1 := s.GetState()

	// Start again should be idempotent
	state2, _ := s.StartRecord()

	if state1.Mode != state2.Mode {
		t.Errorf("expected same mode, got %s and %s", state1.Mode, state2.Mode)
	}

	// Cleanup
	s.Stop()
}

func TestStop(t *testing.T) {
	s := NewService()

	// Init first (creates PTYs)
	_, err := s.Init()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Stop
	state := s.Stop()

	if state.Mode != string(api.ModeIdle) {
		t.Errorf("expected mode idle, got %s", state.Mode)
	}

	// Port info should be preserved after stop (PTYs are not closed)
	if state.UpperPort == "" {
		t.Error("expected non-empty upperPort after stop")
	}
}

func TestStop_Idempotent(t *testing.T) {
	s := NewService()

	// Stop without starting
	state1 := s.Stop()

	// Stop again
	state2 := s.Stop()

	if state1.Mode != state2.Mode {
		t.Errorf("expected same mode")
	}
}

func TestClear(t *testing.T) {
	s := NewService()

	// Add some baseline data
	s.mu.Lock()
	s.baseline = []api.Record{
		{Index: 0, Dir: "TX", Data: "c000", Size: 2},
	}
	s.stats.TX = 1
	s.mu.Unlock()

	// Clear
	state := s.Clear()

	if len(state.Baseline) != 0 {
		t.Errorf("expected empty baseline, got %d", len(state.Baseline))
	}

	if state.Stats.TX != 0 {
		t.Errorf("expected zero stats, got %d", state.Stats.TX)
	}
}

func TestClear_WhileRunning(t *testing.T) {
	s := NewService()

	// Init first to create PTYs
	_, err := s.Init()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Clear should keep record mode
	state := s.Clear()

	if state.Mode != string(api.ModeRecord) {
		t.Errorf("expected mode record, got %s", state.Mode)
	}

	if len(state.Baseline) != 0 {
		t.Errorf("expected empty baseline, got %d", len(state.Baseline))
	}
}

func TestStartCompare_NoBaseline(t *testing.T) {
	s := NewService()

	// No baseline, should return idle
	state, err := s.StartCompare()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Mode != string(api.ModeIdle) {
		t.Errorf("expected mode idle (no baseline), got %s", state.Mode)
	}
}

func TestStartCompare_WithBaseline(t *testing.T) {
	s := NewService()

	// Add baseline
	s.mu.Lock()
	s.baseline = []api.Record{
		{Index: 0, Dir: "TX", Data: hex.EncodeToString([]byte{0xC0, 0x00}), Size: 2},
		{Index: 1, Dir: "RX", Data: hex.EncodeToString([]byte{0xC0, 0x01}), Size: 2},
	}
	s.mu.Unlock()

	state, err := s.StartCompare()
	if err != nil {
		t.Fatalf("startCompare failed: %v", err)
	}

	if state.Mode != string(api.ModeCompare) {
		t.Errorf("expected mode compare, got %s", state.Mode)
	}

	// Cleanup
	s.Stop()
}

func TestEventCallback_Record(t *testing.T) {
	s := NewService()

	var mu sync.Mutex
	var recordEvent api.Record
	eventCount := 0

	s.SetEventCallback(func(event string, data interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if event == "record" {
			recordEvent = data.(api.Record)
			eventCount++
		}
	})

	// Init and start record
	s.Init()
	s.StartRecord()

	// Manually trigger a record event by simulating the bridgeLoop behavior
	s.mu.Lock()
	s.session.Add(session.TX, []byte{0xC0, 0x00}) // TX
	record := api.Record{
		Index: len(s.baseline),
		Dir:   "TX",
		Data:  hex.EncodeToString([]byte{0xC0, 0x00}),
		Size:  2,
	}
	s.baseline = append(s.baseline, record)
	s.stats.TX++
	s.mu.Unlock()

	// Emit event
	if s.onEvent != nil {
		s.onEvent("record", record)
	}

	// Wait for event
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if eventCount != 1 {
		t.Errorf("expected 1 record event, got %d", eventCount)
	}
	if recordEvent.Dir != "TX" {
		t.Errorf("expected TX, got %s", recordEvent.Dir)
	}
	mu.Unlock()

	// Cleanup
	s.Stop()
}

func TestEventCallback_Stats(t *testing.T) {
	s := NewService()

	var mu sync.Mutex
	var statsEvent api.Stats

	s.SetEventCallback(func(event string, data interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if event == "stats" {
			statsEvent = data.(api.Stats)
		}
	})

	// Init
	s.Init()

	// Manually trigger stats event
	s.mu.Lock()
	s.stats.Matched = 5
	s.stats.Diff = 2
	s.mu.Unlock()

	if s.onEvent != nil {
		s.onEvent("stats", s.stats)
	}

	// Wait for event
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if statsEvent.Matched != 5 {
		t.Errorf("expected 5 matched, got %d", statsEvent.Matched)
	}
	if statsEvent.Diff != 2 {
		t.Errorf("expected 2 diff, got %d", statsEvent.Diff)
	}
	mu.Unlock()

	// Cleanup
	s.Stop()
}

func TestGetStateLocked_NoDeadlock(t *testing.T) {
	s := NewService()

	// This tests that GetState doesn't deadlock when called from a method that already holds the lock
	done := make(chan bool)
	go func() {
		s.mu.Lock()
		state := s.getStateLocked()
		s.mu.Unlock()
		done <- state.Mode == string(api.ModeIdle)
	}()

	select {
	case ok := <-done:
		if !ok {
			t.Error("getStateLocked returned wrong state")
		}
	case <-time.After(1 * time.Second):
		t.Error("deadlock detected")
	}
}

func TestFullRecordCycle(t *testing.T) {
	s := NewService()

	// Init
	_, err := s.Init()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Start record
	state, err := s.StartRecord()
	if err != nil {
		t.Fatalf("startRecord failed: %v", err)
	}

	if state.Mode != string(api.ModeRecord) {
		t.Errorf("expected mode record, got %s", state.Mode)
	}

	// Stop
	state = s.Stop()
	if state.Mode != string(api.ModeIdle) {
		t.Errorf("expected mode idle, got %s", state.Mode)
	}

	// Clear
	state = s.Clear()
	if len(state.Baseline) != 0 {
		t.Errorf("expected empty baseline")
	}
}
