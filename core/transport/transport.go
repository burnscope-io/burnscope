// Package transport 定义传输层接口
package transport

import (
	"io"
)

// Transport 传输接口
type Transport interface {
	io.ReadWriteCloser
	// Name 返回传输名称
	Name() string
}

// ControlSignal 控制信号接口（DTR/RTS）
type ControlSignal interface {
	SetDTR(level bool) error
	SetRTS(level bool) error
	GetDTR() bool
	GetRTS() bool
}
