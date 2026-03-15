package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/burnscope-io/burnscope/core/api"
	"github.com/burnscope-io/burnscope/core/service"
)

// App application
type App struct {
	ctx     context.Context
	service *service.Service
}

// NewApp creates a new app
func NewApp() *App {
	return &App{
		service: service.NewService(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	// Set event callback
	a.service.SetEventCallback(func(event string, data interface{}) {
		runtime.EventsEmit(a.ctx, event, data)
	})
}

// Init initializes the application
func (a *App) Init() api.State {
	state, _ := a.service.Init()
	return state
}

// RefreshPorts refreshes the port list
func (a *App) RefreshPorts() api.State {
	return a.service.RefreshPorts()
}

// StartRecord starts recording mode
func (a *App) StartRecord() api.State {
	state, _ := a.service.StartRecord()
	return state
}

// StartCompare starts compare mode
func (a *App) StartCompare() api.State {
	state, _ := a.service.StartCompare()
	return state
}

// Stop stops the current mode
func (a *App) Stop() api.State {
	return a.service.Stop()
}

// Clear clears the baseline data
func (a *App) Clear() api.State {
	return a.service.Clear()
}

// GetState returns the current state
func (a *App) GetState() api.State {
	return a.service.GetState()
}

// SelectLowerPort selects a lower port for communication
func (a *App) SelectLowerPort(portPath string) api.State {
	return a.service.SelectLowerPort(portPath)
}