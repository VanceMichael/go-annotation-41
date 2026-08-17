// Package sweep 负责对截获记录做批量复核。
package sweep

import (
	"context"
	"fmt"
	"sync"
	"time"

	"portbio/internal/model"
)

// Finding 是一条复核结论。
type Finding struct {
	InterceptionID string `json:"interception_id"`
	// Flagged 报告该条记录是否需要人工复核。
	Flagged bool `json:"flagged"`
	// Note 是复核说明。
	Note string `json:"note"`
}

// Sweeper 逐条复核截获记录。
type Sweeper struct {
	mu sync.Mutex
	// PerItem 是单条记录的复核耗时。
	PerItem  time.Duration
	findings []Finding
	checked  int
}

// New 构造复核器。
func New(perItem time.Duration) *Sweeper {
	return &Sweeper{PerItem: perItem}
}

// Checked 返回已复核条数。
func (s *Sweeper) Checked() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checked
}

// Findings 返回复核结论副本。
func (s *Sweeper) Findings() []Finding {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Finding, len(s.findings))
	copy(out, s.findings)
	return out
}

// check 复核单条记录。
func (s *Sweeper) check(it model.Interception) Finding {
	if s.PerItem > 0 {
		time.Sleep(s.PerItem)
	}
	f := Finding{InterceptionID: it.ID}
	if it.Risk.Rank() >= model.RiskHigh.Rank() && it.Measure == model.MeasureDisinfect {
		f.Flagged = true
		f.Note = "高风险仅消杀后放行，需人工复核"
	}
	if it.Risk == model.RiskQuarantine && it.Measure == model.MeasureRelease {
		f.Flagged = true
		f.Note = "检疫性有害生物不得放行"
	}
	s.mu.Lock()
	s.findings = append(s.findings, f)
	s.checked++
	s.mu.Unlock()
	return f
}

// Sweep 逐条复核一批截获记录。
//
// 每处理一条记录之前都必须检查调用方 context 是否已结束：
// 已结束时立即停止并返回已完成条数与可沿错误链判定的 context 错误，
// 不得把剩余记录继续跑完。
func (s *Sweeper) Sweep(ctx context.Context, items []model.Interception) (int, error) {
	done := 0
	for i, it := range items {
		if err := ctx.Err(); err != nil {
			return done, fmt.Errorf("%w: 在第 %d/%d 条处停止: %v",
				model.ErrSweepAborted, i+1, len(items), err)
		}
		s.check(it)
		done++
	}
	return done, nil
}

// Report 是一次批量复核的结果摘要。
type Report struct {
	// Submitted 是提交复核的记录条数。
	Submitted int `json:"submitted"`
	// Checked 是实际复核完成的条数。
	Checked int `json:"checked"`
	// Flagged 是需要人工复核的条数。
	Flagged int `json:"flagged"`
	// Aborted 报告本次复核是否被调用方中止。
	Aborted bool `json:"aborted"`
	// ElapsedMS 是本次复核实际耗时毫秒数。
	ElapsedMS int64 `json:"elapsed_ms"`
	// Message 是中止原因，未中止时为空。
	Message string `json:"message,omitempty"`
}

// SweepBatch 复核一批记录并生成结果摘要。
func (s *Sweeper) SweepBatch(ctx context.Context, items []model.Interception) Report {
	start := time.Now()
	done, err := s.Sweep(ctx, items)
	elapsed := time.Since(start)

	flagged := 0
	for _, f := range s.Findings() {
		if f.Flagged {
			flagged++
		}
	}

	rep := Report{
		Submitted: len(items),
		Checked:   done,
		Flagged:   flagged,
		Aborted:   err != nil,
		ElapsedMS: elapsed.Milliseconds(),
	}
	if err != nil {
		rep.Message = err.Error()
	}
	return rep
}

// Describe 返回复核结论的单行描述。
func Describe(f Finding) string {
	if !f.Flagged {
		return fmt.Sprintf("%s 通过", f.InterceptionID)
	}
	return fmt.Sprintf("%s 需复核: %s", f.InterceptionID, f.Note)
}
