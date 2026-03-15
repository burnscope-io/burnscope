//go:build darwin

package main

import (
	"context"
	"testing"

	"github.com/burnscope-io/burnscope/core/api"
)

func TestApp_NewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Error("NewApp returned nil")
	}
}

func TestApp_Stop_Idempotent(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// Stop on non-running app should be safe
	state := app.Stop()
	if state.Mode != string(api.ModeIdle) {
		t.Error("Expected idle mode")
	}
}

func TestApp_Clear(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	state := app.Clear()

	// Clear without PTY returns idle mode
	// (PTYs are created on Init)
	if state.Mode != string(api.ModeIdle) {
		t.Errorf("Expected idle mode after clear without PTY, got %s", state.Mode)
	}

	if len(state.Baseline) != 0 {
		t.Error("Expected empty baseline after clear")
	}
}

func TestApp_GetState(t *testing.T) {
	app := NewApp()
	
	state := app.GetState()
	
	if state.Mode != string(api.ModeIdle) {
		t.Error("Expected idle mode")
	}
}
