// Package session 管理会话记录
package session

import (
	"encoding/json"
	"os"
	"time"

	"github.com/burnscope-io/burnscope/internal/protocol"
)

// Record 单条记录
type Record struct {
	Index     int       `json:"index"`
	Direction string    `json:"direction"`
	Name      string    `json:"name"`
	RawData   []byte    `json:"raw_data"`
	Timestamp time.Time `json:"timestamp"`
}

// Session 会话
type Session struct {
	Device    string    `json:"device"`
	BaudRate  int       `json:"baud_rate"`
	Protocol  string    `json:"protocol"`
	StartTime time.Time `json:"start_time"`
	Records   []Record  `json:"records"`
}

// NewSession 创建新会话
func NewSession(device string, baudRate int, protocolName string) *Session {
	return &Session{
		Device:    device,
		BaudRate:  baudRate,
		Protocol:  protocolName,
		StartTime: time.Now(),
		Records:   make([]Record, 0),
	}
}

// AddCommand 添加命令记录
func (s *Session) AddCommand(cmd *protocol.Command) {
	record := Record{
		Index:     cmd.Index,
		Direction: cmd.Direction.String(),
		Name:      cmd.Name,
		RawData:   cmd.RawData,
		Timestamp: time.Now(),
	}
	s.Records = append(s.Records, record)
}

// AddRawData 添加原始数据记录
func (s *Session) AddRawData(direction protocol.Direction, data []byte) {
	record := Record{
		Index:     len(s.Records) + 1,
		Direction: direction.String(),
		Name:      "RAW",
		RawData:   data,
		Timestamp: time.Now(),
	}
	s.Records = append(s.Records, record)
}

// Save 保存会话到文件
func (s *Session) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load 从文件加载会话
func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// Stats 统计信息
type Stats struct {
	TotalCommands int `json:"total_commands"`
	TXCount       int `json:"tx_count"`
	RXCount       int `json:"rx_count"`
}

// GetStats 获取统计信息
func (s *Session) GetStats() Stats {
	stats := Stats{TotalCommands: len(s.Records)}
	for _, r := range s.Records {
		if r.Direction == "TX" {
			stats.TXCount++
		} else {
			stats.RXCount++
		}
	}
	return stats
}

// Clear 清空记录
func (s *Session) Clear() {
	s.Records = make([]Record, 0)
	s.StartTime = time.Now()
}
