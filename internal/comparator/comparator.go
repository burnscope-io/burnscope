// Package comparator 实现命令对比
package comparator

import (
	"bytes"
	"fmt"

	"github.com/burnscope-io/burnscope/internal/session"
)

// Result 对比结果
type Result int

const (
	Match Result = iota // 匹配
	Diff                 // 差异
	Skip                 // 跳过（基准已结束）
)

func (r Result) String() string {
	switch r {
	case Match:
		return "✓"
	case Diff:
		return "✗"
	default:
		return "-"
	}
}

// LineResult 单行对比结果
type LineResult struct {
	Index    int
	Expected *session.Record
	Actual   *session.Record
	Result   Result
	Message  string
}

// Comparator 对比器
type Comparator struct {
	golden      *session.Session
	position    int
	results     []LineResult
	ignoreBytes map[string]bool // 命令中需要忽略的字节位置
}

// NewComparator 创建对比器
func NewComparator(golden *session.Session) *Comparator {
	return &Comparator{
		golden:   golden,
		position: 0,
		results:  make([]LineResult, 0),
		ignoreBytes: map[string]bool{
			// 序列号等动态字段在协议解析时处理
		},
	}
}

// Compare 对比单条记录
func (c *Comparator) Compare(actual *session.Record) LineResult {
	result := LineResult{
		Index:  c.position + 1,
		Actual: actual,
	}

	if c.position >= len(c.golden.Records) {
		result.Result = Skip
		result.Message = "基准已结束"
		c.results = append(c.results, result)
		return result
	}

	expected := &c.golden.Records[c.position]
	result.Expected = expected

	// 对比方向
	if actual.Direction != expected.Direction {
		result.Result = Diff
		result.Message = fmt.Sprintf("方向不匹配: 期望 %s, 实际 %s", expected.Direction, actual.Direction)
		c.results = append(c.results, result)
		c.position++
		return result
	}

	// 对比名称
	if actual.Name != expected.Name {
		result.Result = Diff
		result.Message = fmt.Sprintf("命令不匹配: 期望 %s, 实际 %s", expected.Name, actual.Name)
		c.results = append(c.results, result)
		c.position++
		return result
	}

	// 对比数据（使用智能匹配）
	if !c.compareData(expected, actual) {
		result.Result = Diff
		result.Message = fmt.Sprintf("数据不匹配: 期望 %d 字节, 实际 %d 字节", len(expected.RawData), len(actual.RawData))
		c.results = append(c.results, result)
		c.position++
		return result
	}

	result.Result = Match
	result.Message = ""
	c.results = append(c.results, result)
	c.position++
	return result
}

// compareData 智能对比数据
func (c *Comparator) compareData(expected, actual *session.Record) bool {
	// 长度必须匹配
	if len(expected.RawData) != len(actual.RawData) {
		return false
	}

	// 原始字节对比
	// TODO: 未来可以根据命令类型忽略动态字段（如序列号）
	return bytes.Equal(expected.RawData, actual.RawData)
}

// Stats 获取统计
func (c *Comparator) Stats() (matched, diff, total int) {
	for _, r := range c.results {
		total++
		switch r.Result {
		case Match:
			matched++
		case Diff:
			diff++
		}
	}
	return
}

// Reset 重置对比器
func (c *Comparator) Reset() {
	c.position = 0
	c.results = make([]LineResult, 0)
}

// GetResults 获取所有结果
func (c *Comparator) GetResults() []LineResult {
	return c.results
}

// IsComplete 检查是否完��
func (c *Comparator) IsComplete() bool {
	return c.position >= len(c.golden.Records)
}

// Progress 获取进度
func (c *Comparator) Progress() (current, total int) {
	return c.position, len(c.golden.Records)
}

// SetPosition 设置位置（用于跳过某些记录）
func (c *Comparator) SetPosition(pos int) {
	if pos >= 0 && pos <= len(c.golden.Records) {
		c.position = pos
	}
}

// Remaining 获取剩余待对比数量
func (c *Comparator) Remaining() int {
	return len(c.golden.Records) - c.position
}