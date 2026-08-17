// Package notify 负责疫情通报上报与失败重试。
package notify

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"portbio/internal/model"
)

// MaxAttempts 是通报的最大尝试次数。
const MaxAttempts = 5

// Backoff 是每次重试之间的退避基数。
const Backoff = 2 * time.Millisecond

// Attempt 记录一次通报尝试。
type Attempt struct {
	No int `json:"no"`
	// OK 报告本次尝试是否成功。
	OK bool `json:"ok"`
	// Retryable 报告本次失败是否被判定为可重试。
	Retryable bool `json:"retryable"`
	// Message 是失败信息，成功时为空。
	Message string `json:"message,omitempty"`
	// BackoffMS 是本次失败后的退避毫秒数。
	BackoffMS int64 `json:"backoff_ms,omitempty"`
}

// Outcome 是一次通报的最终结果。
type Outcome struct {
	InterceptionID string    `json:"interception_id"`
	OK             bool      `json:"ok"`
	Attempts       []Attempt `json:"attempts"`
	// Calls 是实际发起的通道调用次数。
	Calls int `json:"calls"`
	// Retries 是重试次数。
	Retries int `json:"retries"`
	// Message 是最终失败信息，成功时为空。
	Message string `json:"message,omitempty"`
}

// Channel 是通报通道。
type Channel interface {
	// Send 发送一条通报。
	Send(interceptionID string) error
}

// Retryable 判定一次通报失败是否可以重试。
//
// 只有通道调用频次超限属于可重试错误，必须沿错误链用 errors.Is 判定；
// 通道已停用、内容被拒收都是确定性错误，不得重试。
func Retryable(err error) bool {
	return errors.Is(err, model.ErrNotifyThrottled)
}

// Reporter 按退避重试策略上报通报。
type Reporter struct {
	mu sync.Mutex
	ch Channel
}

// NewReporter 构造通报上报器。
func NewReporter(ch Channel) *Reporter {
	return &Reporter{ch: ch}
}

// Report 上报一条通报，必要时按退避重试。
//
// 可重试错误最多尝试 MaxAttempts 次；确定性错误立即返回，不做任何重试。
func (r *Reporter) Report(interceptionID string) Outcome {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := Outcome{InterceptionID: interceptionID, Attempts: make([]Attempt, 0, MaxAttempts)}
	var last error

	for n := 1; n <= MaxAttempts; n++ {
		out.Calls++
		err := r.ch.Send(interceptionID)
		if err == nil {
			out.Attempts = append(out.Attempts, Attempt{No: n, OK: true})
			out.OK = true
			return out
		}

		last = err
		retryable := Retryable(err)
		a := Attempt{No: n, OK: false, Retryable: retryable, Message: err.Error()}

		if !retryable {
			out.Attempts = append(out.Attempts, a)
			out.Message = err.Error()
			return out
		}
		if n == MaxAttempts {
			out.Attempts = append(out.Attempts, a)
			out.Message = err.Error()
			return out
		}

		wait := Backoff * time.Duration(n)
		a.BackoffMS = wait.Milliseconds()
		out.Attempts = append(out.Attempts, a)
		out.Retries++
		time.Sleep(wait)
	}

	if last != nil {
		out.Message = last.Error()
	}
	return out
}

// Classify 把通报结果映射为哨兵错误，供调用方判定。
func Classify(out Outcome) error {
	if out.OK {
		return nil
	}
	if len(out.Attempts) == 0 {
		return fmt.Errorf("%w: 通报 %s 未发起任何尝试",
			model.ErrNotifyRejected, out.InterceptionID)
	}
	last := out.Attempts[len(out.Attempts)-1]
	if last.Retryable {
		return fmt.Errorf("%w: 通报 %s 重试 %d 次后仍失败",
			model.ErrNotifyThrottled, out.InterceptionID, out.Retries)
	}
	return fmt.Errorf("%w: 通报 %s 失败: %s",
		model.ErrNotifyChannelDisabled, out.InterceptionID, last.Message)
}

// ThrottledChannel 是演练用通道：前 limit 次调用返回频次超限，之后成功。
type ThrottledChannel struct {
	mu    sync.Mutex
	limit int
	calls int
}

// NewThrottledChannel 构造限流演练通道。
func NewThrottledChannel(limit int) *ThrottledChannel {
	return &ThrottledChannel{limit: limit}
}

// Send 实现 Channel。
func (c *ThrottledChannel) Send(string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.calls <= c.limit {
		return fmt.Errorf("%w: 每分钟仅允许 %d 次", model.ErrNotifyThrottled, c.limit)
	}
	return nil
}

// Calls 返回已发起的调用次数。
func (c *ThrottledChannel) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// DisabledChannel 是演练用通道：始终返回通道已停用。
type DisabledChannel struct {
	mu    sync.Mutex
	calls int
}

// NewDisabledChannel 构造停用演练通道。
func NewDisabledChannel() *DisabledChannel {
	return &DisabledChannel{}
}

// Send 实现 Channel。
func (c *DisabledChannel) Send(string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return fmt.Errorf("%w: 请联系上级系统管理员", model.ErrNotifyChannelDisabled)
}

// Calls 返回已发起的调用次数。
func (c *DisabledChannel) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// Describe 返回通报结果的单行描述。
func Describe(out Outcome) string {
	if out.OK {
		return fmt.Sprintf("%s 通报成功（调用%d次, 重试%d次）",
			out.InterceptionID, out.Calls, out.Retries)
	}
	return fmt.Sprintf("%s 通报失败（调用%d次, 重试%d次）: %s",
		out.InterceptionID, out.Calls, out.Retries, out.Message)
}
