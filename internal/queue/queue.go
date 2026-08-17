// Package queue 负责查验队列的排序与分页。
package queue

import (
	"fmt"
	"sync"

	"portbio/internal/model"
)

// Queue 是待查验截获记录的队列。
type Queue struct {
	mu    sync.RWMutex
	byID  map[string]model.Interception
	order []string
}

// New 构造查验队列。
func New() *Queue {
	return &Queue{byID: make(map[string]model.Interception)}
}

// Add 入队一条截获记录。
func (q *Queue) Add(it model.Interception) error {
	if err := it.Validate(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, dup := q.byID[it.ID]; !dup {
		q.order = append(q.order, it.ID)
	}
	q.byID[it.ID] = it
	return nil
}

// AddAll 批量入队。
func (q *Queue) AddAll(items []model.Interception) error {
	for _, it := range items {
		if err := q.Add(it); err != nil {
			return err
		}
	}
	return nil
}

// Len 返回队列长度。
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.byID)
}

// Get 按编号查询。
func (q *Queue) Get(id string) (model.Interception, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	it, ok := q.byID[id]
	if !ok {
		return model.Interception{}, fmt.Errorf("%w: %s", model.ErrInterceptionNotFound, id)
	}
	return it, nil
}

// Ordered 返回按查验口径排序后的全部记录。
//
// 排序口径是「风险等级降序、受理时间升序、编号升序」，
// 同一份队列多次调用必须得到完全一致的顺序。
func (q *Queue) Ordered() []model.Interception {
	q.mu.RLock()
	out := make([]model.Interception, 0, len(q.byID))
	for _, id := range q.order {
		out = append(out, q.byID[id])
	}
	q.mu.RUnlock()

	model.SortInterceptions(out)
	return out
}

// Page 返回查验队列的第 page 页，页码从 1 开始。
//
// 分页建立在 Ordered 的稳定顺序之上：逐页取完必须恰好覆盖全部记录，
// 既不重复也不遗漏。页码越界时返回空页。
func (q *Queue) Page(page, size int) []model.Interception {
	if page <= 0 || size <= 0 {
		return nil
	}
	all := q.Ordered()
	start := (page - 1) * size
	if start >= len(all) {
		return nil
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.Interception, end-start)
	copy(out, all[start:end])
	return out
}

// Pages 返回按给定页大小分页后的总页数。
func (q *Queue) Pages(size int) int {
	if size <= 0 {
		return 0
	}
	n := q.Len()
	if n == 0 {
		return 0
	}
	return (n + size - 1) / size
}

// Walk 逐页遍历整个队列并返回收集到的记录编号。
func (q *Queue) Walk(size int) []string {
	out := make([]string, 0, q.Len())
	for p := 1; p <= q.Pages(size); p++ {
		for _, it := range q.Page(p, size) {
			out = append(out, it.ID)
		}
	}
	return out
}

// Coverage 描述一次逐页遍历的覆盖情况。
type Coverage struct {
	// Total 是队列内记录总数。
	Total int `json:"total"`
	// Collected 是逐页收集到的记录条数。
	Collected int `json:"collected"`
	// Distinct 是收集到的互异编号数。
	Distinct int `json:"distinct"`
	// Duplicated 是被重复取到的编号。
	Duplicated []string `json:"duplicated"`
	// Missing 是一次都没被取到的编号。
	Missing []string `json:"missing"`
	// Complete 报告本次遍历是否恰好覆盖全部记录。
	Complete bool `json:"complete"`
}

// CheckCoverage 逐页遍历队列并核对覆盖情况。
func (q *Queue) CheckCoverage(size int) Coverage {
	collected := q.Walk(size)

	seen := make(map[string]int, len(collected))
	for _, id := range collected {
		seen[id]++
	}

	cov := Coverage{
		Total:      q.Len(),
		Collected:  len(collected),
		Distinct:   len(seen),
		Duplicated: make([]string, 0, 2),
		Missing:    make([]string, 0, 2),
	}

	for _, it := range q.Ordered() {
		switch seen[it.ID] {
		case 0:
			cov.Missing = append(cov.Missing, it.ID)
		case 1:
		default:
			cov.Duplicated = append(cov.Duplicated, it.ID)
		}
	}
	cov.Complete = cov.Collected == cov.Total &&
		cov.Distinct == cov.Total &&
		len(cov.Duplicated) == 0 && len(cov.Missing) == 0
	return cov
}

// Verify 校验分页遍历的完整性。
func (q *Queue) Verify(size int) error {
	cov := q.CheckCoverage(size)
	if !cov.Complete {
		return fmt.Errorf("%w: 队列 %d 条, 逐页取到 %d 条（互异 %d 条）, 重复 %v, 遗漏 %v",
			model.ErrQueuePagination, cov.Total, cov.Collected, cov.Distinct,
			cov.Duplicated, cov.Missing)
	}
	return nil
}

// CountByRisk 按风险等级统计队列内记录数。
func (q *Queue) CountByRisk() map[model.Risk]int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make(map[model.Risk]int)
	for _, r := range model.AllRisks() {
		out[r] = 0
	}
	for _, it := range q.byID {
		out[it.Risk]++
	}
	return out
}

// Describe 返回截获记录的单行描述。
func Describe(it model.Interception) string {
	return fmt.Sprintf("%s %s %s %s %s 数量%d 受理%s",
		it.ID, it.PortCode, it.SpeciesCode, it.Channel.DisplayName(),
		it.Risk.DisplayName(), it.Quantity,
		it.AcceptedAt.UTC().Format("2006-01-02 15:04"))
}
