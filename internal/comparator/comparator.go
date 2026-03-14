// Package comparator 实现字节对比
package comparator

import (
	"bytes"

	"github.com/burnscope-io/burnscope/internal/session"
)

// Result 对比结果
type Result int

const (
	Match Result = iota
	Diff
	Skip
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
}

// Comparator 对比器
type Comparator struct {
	golden   *session.Session
	position int
	results  []LineResult
}

// NewComparator 创建对比器
func NewComparator(golden *session.Session) *Comparator {
	return &Comparator{
		golden:   golden,
		position: 0,
		results:  make([]LineResult, 0),
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
		c.results = append(c.results, result)
		return result
	}

	expected := &c.golden.Records[c.position]
	result.Expected = expected

	// 对比方向
	if actual.Direction != expected.Direction {
		result.Result = Diff
		c.results = append(c.results, result)
		c.position++
		return result
	}

	// 对比原始字节
	if !bytes.Equal(actual.Data, expected.Data) {
		result.Result = Diff
		c.results = append(c.results, result)
		c.position++
		return result
	}

	result.Result = Match
	c.results = append(c.results, result)
	c.position++
	return result
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

// Progress 获取进度
func (c *Comparator) Progress() (current, total int) {
	return c.position, len(c.golden.Records)
}
