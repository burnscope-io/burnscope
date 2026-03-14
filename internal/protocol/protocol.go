// Package protocol 定义烧录协议解析器接口
package protocol

// Direction 数据方向
type Direction int

const (
	TX Direction = iota // 主机→设备
	RX                  // 设备→主机
)

func (d Direction) String() string {
	if d == TX {
		return "TX"
	}
	return "RX"
}

// Command 解析后的命令
type Command struct {
	Index     int       // 序号
	Direction Direction // 方向
	Name      string    // 命令名称
	RawData   []byte    // 原始数据
	Parsed    any       // 解析后的结构（可选）
}

// Parser 协议解析器接口
type Parser interface {
	// Parse 解析原始数据，返回命令列表
	Parse(data []byte) []*Command
	// Name 返回协议名称
	Name() string
}

// CommandMatcher 命令匹配器接口（用于对比）
type CommandMatcher interface {
	// Match 判断两个命令是否匹配
	Match(a, b *Command) bool
	// DescribeDiff 描述差异
	DescribeDiff(a, b *Command) string
}
