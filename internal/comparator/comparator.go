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
	Index      int
	ExpectedTX *session.Record
	ExpectedRX *session.Record // 对应的 RX 响应
	Actual     *session.Record
	Result     Result
}

// Comparator 对比器
// 假设基准格式为: TX, RX, TX, RX, ...
// 对比时只对比 TX，RX 用于回放
type Comparator struct {
	golden   *session.Session
	position int // 当前 TX 位置
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

// Compare 对比 TX 记录
// 返回结果中包��期望的 TX 和对应的 RX 响应
func (c *Comparator) Compare(actual *session.Record) LineResult {
	result := LineResult{
		Index:  len(c.results) + 1,
		Actual: actual,
	}

	// 找到下一个 TX 记录
	expectedTX, expectedRX := c.findNextTXRX()

	if expectedTX == nil {
		result.Result = Skip
		c.results = append(c.results, result)
		return result
	}

	result.ExpectedTX = expectedTX
	result.ExpectedRX = expectedRX

	// 对比原始字节
	if !bytes.Equal(actual.Data, expectedTX.Data) {
		result.Result = Diff
		c.results = append(c.results, result)
		return result
	}

	result.Result = Match
	c.results = append(c.results, result)
	return result
}

// findNextTXRX 找到下一个 TX 和对应的 RX
func (c *Comparator) findNextTXRX() (*session.Record, *session.Record) {
	// 跳过 RX 记录，找到下一个 TX
	for c.position < len(c.golden.Records) {
		record := &c.golden.Records[c.position]
		if record.Direction == session.TX {
			// 找到了 TX，检查是否有对应的 RX
			c.position++
			var rx *session.Record
			if c.position < len(c.golden.Records) && c.golden.Records[c.position].Direction == session.RX {
				rx = &c.golden.Records[c.position]
				c.position++
			}
			return record, rx
		}
		c.position++
	}
	return nil, nil
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

// Progress 获取进度 (TX 记录数)
func (c *Comparator) Progress() (current, total int) {
	// 计算 TX 总数
	txCount := 0
	for _, r := range c.golden.Records {
		if r.Direction == session.TX {
			txCount++
		}
	}
	return len(c.results), txCount
}