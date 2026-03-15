// Package api defines the types for frontend-backend communication
package api

// State represents the complete application state
type State struct {
	Mode       string     `json:"mode"`       // "", "record", "compare"
	UpperPort  string     `json:"upperPort"`  
	LowerPorts []PortInfo `json:"lowerPorts"` 
	Baseline   []Record   `json:"baseline"`   
	Actual     []Record   `json:"actual"`     
	Stats      Stats      `json:"stats"`      
}

// PortInfo represents a serial port
type PortInfo struct {
	PortPath string `json:"portPath"` // "/dev/ttys000"
	PortType string `json:"portType"` // "virtual", "physical"
}

// Record represents a data record
type Record struct {
	Index int    `json:"index"`      
	Dir   string `json:"dir"`        // "TX", "RX"
	Data  string `json:"data"`       // hex encoded
	Size  int    `json:"size"`       
	Match *bool  `json:"match,omitempty"` // only for actual
}

// Stats represents statistics
type Stats struct {
	TX      int `json:"tx"`
	RX      int `json:"rx"`
	Matched int `json:"matched"`
	Diff    int `json:"diff"`
}

// Direction represents data direction
type Direction string

const (
	TX Direction = "TX"
	RX Direction = "RX"
)

// Mode represents application mode
type Mode string

const (
	ModeIdle    Mode = ""
	ModeRecord  Mode = "record"
	ModeCompare Mode = "compare"
)
