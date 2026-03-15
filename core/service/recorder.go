// Package service provides the core business logic
package service

import (
	"encoding/hex"
	"sync"

	"github.com/burnscope-io/burnscope/core/api"
	"github.com/burnscope-io/burnscope/core/comparator"
	"github.com/burnscope-io/burnscope/core/session"
	"github.com/burnscope-io/burnscope/core/transport"
)

// EventCallback is called when state changes
type EventCallback func(event string, data interface{})

// LowerConn represents a lower port connection
type LowerConn struct {
	PortPath string
	PortType string // "virtual" or "physical"
	Conn     transport.Transport
}

// Service provides the core recording and comparison functionality
type Service struct {
	mu sync.Mutex

	// State
	mode       api.Mode
	upperPort  string
	lowerPorts []api.PortInfo
	baseline   []api.Record
	actual     []api.Record
	stats      api.Stats

	// Internal
	upperPty *transport.PtyTransport
	lowerPty *transport.PtyTransport // Virtual PTY, always open

	// Lower connection pool
	lowerConnPool  map[string]*LowerConn // portPath -> connection
	currentLower   string                 // currently selected lower port path

	session    *session.Session
	comparator *comparator.Comparator

	// Callback
	onEvent EventCallback
}

// NewService creates a new service
func NewService() *Service {
	return &Service{
		mode:          api.ModeIdle,
		lowerPorts:    []api.PortInfo{},
		baseline:      []api.Record{},
		actual:        []api.Record{},
		lowerConnPool: make(map[string]*LowerConn),
	}
}

// SetEventCallback sets the event callback
func (s *Service) SetEventCallback(cb EventCallback) {
	s.onEvent = cb
}

// SetBaseline sets the baseline data for testing
func (s *Service) SetBaseline(records []api.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baseline = records
}

// GetState returns the current state
func (s *Service) GetState() api.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getStateLocked()
}

// getStateLocked returns state without locking (must be called with lock held)
func (s *Service) getStateLocked() api.State {
	return api.State{
		Mode:       string(s.mode),
		UpperPort:  s.upperPort,
		LowerPorts: s.lowerPorts,
		Baseline:   s.baseline,
		Actual:     s.actual,
		Stats:      s.stats,
	}
}

// Init initializes the service
func (s *Service) Init() (api.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create upper PTY
	upper, err := transport.NewPtyTransport()
	if err != nil {
		return s.getStateLocked(), err
	}
	s.upperPty = upper
	s.upperPort = upper.SlavePath()

	// Create lower PTY (always open)
	lower, err := transport.NewPtyTransport()
	if err != nil {
		return s.getStateLocked(), err
	}
	s.lowerPty = lower

	// Add virtual PTY to connection pool
	virtualPath := lower.SlavePath()
	s.lowerConnPool[virtualPath] = &LowerConn{
		PortPath: virtualPath,
		PortType: "virtual",
		Conn:     lower,
	}

	// Build lower ports list (virtual first)
	s.lowerPorts = []api.PortInfo{
		{PortPath: virtualPath, PortType: "virtual"},
	}

	// Add physical ports
	physical, _ := transport.ListPorts()
	for _, p := range physical {
		s.lowerPorts = append(s.lowerPorts, api.PortInfo{
			PortPath: p,
			PortType: "physical",
		})
	}

	// Default to virtual PTY
	s.currentLower = virtualPath

	// Initialize session
	s.session = session.NewSession("record", 0)
	s.mode = api.ModeRecord

	// Start the main loop
	go s.loop()

	return s.getStateLocked(), nil
}

// RefreshPorts refreshes the port list
func (s *Service) RefreshPorts() api.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Keep virtual port, refresh physical
	var ports []api.PortInfo
	if s.lowerPty != nil {
		ports = append(ports, api.PortInfo{
			PortPath: s.lowerPty.SlavePath(),
			PortType: "virtual",
		})
	}

	physical, _ := transport.ListPorts()
	for _, p := range physical {
		ports = append(ports, api.PortInfo{
			PortPath: p,
			PortType: "physical",
		})
	}
	s.lowerPorts = ports

	return s.getStateLocked()
}

// StartRecord starts recording mode
func (s *Service) StartRecord() (api.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reset recording state (keep baseline for appending)
	s.session = session.NewSession("record", 0)
	s.actual = []api.Record{}
	s.stats.Matched = 0
	s.stats.Diff = 0
	s.mode = api.ModeRecord

	return s.getStateLocked(), nil
}

// StartCompare starts compare mode
func (s *Service) StartCompare() (api.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check baseline exists
	if len(s.baseline) == 0 {
		return s.getStateLocked(), nil
	}

	// Convert baseline to session for comparator
	sess := session.NewSession("compare", 0)
	for _, r := range s.baseline {
		data, _ := hex.DecodeString(r.Data)
		sess.Add(session.Direction(r.Dir), data)
	}

	s.comparator = comparator.NewComparator(sess)
	s.actual = []api.Record{}
	s.stats.Matched = 0
	s.stats.Diff = 0
	s.mode = api.ModeCompare

	return s.getStateLocked(), nil
}

// Stop stops the current mode (returns to idle)
func (s *Service) Stop() api.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mode = api.ModeIdle
	return s.getStateLocked()
}

// Clear clears the baseline data (keeps PTYs alive)
func (s *Service) Clear() api.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear data only
	s.baseline = []api.Record{}
	s.actual = []api.Record{}
	s.stats = api.Stats{}

	// Reset session and comparator
	s.session = session.NewSession("record", 0)
	s.comparator = nil

	// Switch to record mode only if PTYs exist
	if s.upperPty != nil && s.lowerPty != nil {
		s.mode = api.ModeRecord
	}

	return s.getStateLocked()
}

// loop is the main loop that handles both record and compare modes
func (s *Service) loop() {
	txChan := make(chan []byte, 100)
	rxChan := make(chan []byte, 100)
	lowerChangeChan := make(chan struct{}, 1)

	// TX reader: upper -> internal
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := s.upperPty.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				txChan <- data
			}
		}
	}()

	// RX reader goroutine (restarted when lower port changes)
	var rxStopChan chan struct{}
	startRXReader := func() {
		if rxStopChan != nil {
			close(rxStopChan)
		}
		rxStopChan = make(chan struct{})
		
		s.mu.Lock()
		conn := s.getCurrentLowerConnLocked()
		s.mu.Unlock()
		
		if conn == nil {
			return
		}
		
		go func(stopChan chan struct{}) {
			buf := make([]byte, 4096)
			for {
				select {
				case <-stopChan:
					return
				default:
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					if n > 0 {
						data := make([]byte, n)
						copy(data, buf[:n])
						select {
						case rxChan <- data:
						case <-stopChan:
							return
						}
					}
				}
			}
		}(rxStopChan)
	}

	// Start initial RX reader
	startRXReader()

	// Main processing loop
	for {
		select {
		case txData := <-txChan:
			s.handleTX(txData)
		case rxData := <-rxChan:
			s.handleRX(rxData)
		case <-lowerChangeChan:
			startRXReader()
		}
	}
}

// getCurrentLowerConnLocked returns the current lower connection (must hold lock)
func (s *Service) getCurrentLowerConnLocked() transport.Transport {
	if s.currentLower == "" {
		return nil
	}
	conn, ok := s.lowerConnPool[s.currentLower]
	if !ok {
		return nil
	}
	return conn.Conn
}

// SelectLowerPort selects a lower port for communication
func (s *Service) SelectLowerPort(portPath string) api.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already selected
	if s.currentLower == portPath {
		return s.getStateLocked()
	}

	// Find port info
	var portType string
	for _, p := range s.lowerPorts {
		if p.PortPath == portPath {
			portType = p.PortType
			break
		}
	}

	// If physical port, open connection
	if portType == "physical" {
		// Check if already in pool
		if _, ok := s.lowerConnPool[portPath]; !ok {
			conn, err := transport.NewSerialTransport(portPath, 115200)
			if err != nil {
				// Failed to open, stay with current
				return s.getStateLocked()
			}
			s.lowerConnPool[portPath] = &LowerConn{
				PortPath: portPath,
				PortType: "physical",
				Conn:     conn,
			}
		}
	}

	s.currentLower = portPath
	return s.getStateLocked()
}

// handleTX handles TX data based on current mode
func (s *Service) handleTX(data []byte) {
	s.mu.Lock()
	mode := s.mode
	conn := s.getCurrentLowerConnLocked()
	s.mu.Unlock()

	if conn == nil {
		return
	}

	switch mode {
	case api.ModeRecord:
		// Record mode: forward to lower and record
		conn.Write(data)
		s.recordData(session.TX, data)

	case api.ModeCompare:
		// Compare mode: compare with baseline and replay RX
		s.compareTX(data)

	case api.ModeIdle:
		// Idle: do nothing
	}
}

// handleRX handles RX data based on current mode
func (s *Service) handleRX(data []byte) {
	s.mu.Lock()
	mode := s.mode
	s.mu.Unlock()

	switch mode {
	case api.ModeRecord:
		// Record mode: forward to upper and record
		s.upperPty.Write(data)
		s.recordData(session.RX, data)

	case api.ModeCompare:
		// Compare mode: RX from lower is ignored (we replay from baseline)
		// But in compare mode, lower PTY is not connected, so this shouldn't happen

	case api.ModeIdle:
		// Idle: do nothing
	}
}

// recordData records data to baseline
func (s *Service) recordData(dir session.Direction, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.session.Add(dir, data)

	record := api.Record{
		Index: len(s.baseline),
		Dir:   string(dir),
		Data:  hex.EncodeToString(data),
		Size:  len(data),
	}
	s.baseline = append(s.baseline, record)

	if dir == session.TX {
		s.stats.TX++
	} else {
		s.stats.RX++
	}

	if s.onEvent != nil {
		s.onEvent("record", record)
	}
}

// compareTX compares TX with baseline and replays RX
func (s *Service) compareTX(data []byte) {
	s.mu.Lock()
	cmp := s.comparator
	s.mu.Unlock()

	if cmp == nil {
		return
	}

	actual := &session.Record{
		Direction: session.TX,
		Data:      data,
	}
	result := cmp.Compare(actual)

	s.mu.Lock()
	// Find the baseline TX index
	txIndex := 0
	for i, r := range s.baseline {
		if r.Dir == "TX" {
			if txIndex == result.Index {
				txIndex = i
				break
			}
			txIndex++
		}
	}

	match := result.Result == comparator.Match
	record := api.Record{
		Index: txIndex,
		Dir:   "TX",
		Data:  hex.EncodeToString(data),
		Size:  len(data),
		Match: &match,
	}
	s.actual = append(s.actual, record)

	if match {
		s.stats.Matched++
	} else {
		s.stats.Diff++
	}
	s.mu.Unlock()

	// Emit TX event
	if s.onEvent != nil {
		s.onEvent("record", record)
		s.onEvent("stats", s.stats)
	}

	// Replay all RXs (may be multiple consecutive RXs)
	for _, expectedRX := range result.ExpectedRXs {
		s.upperPty.Write(expectedRX.Data)

		// Find this RX's index in baseline
		rxIndex := txIndex + 1
		for rxIndex < len(s.baseline) && s.baseline[rxIndex].Dir != "RX" {
			rxIndex++
		}

		if rxIndex < len(s.baseline) && s.baseline[rxIndex].Dir == "RX" {
			rxRecord := api.Record{
				Index: rxIndex,
				Dir:   "RX",
				Data:  hex.EncodeToString(expectedRX.Data),
				Size:  len(expectedRX.Data),
				Match: boolPtr(true),
			}
			s.mu.Lock()
			s.actual = append(s.actual, rxRecord)
			s.mu.Unlock()

			if s.onEvent != nil {
				s.onEvent("record", rxRecord)
			}

			txIndex = rxIndex
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}
