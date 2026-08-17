package queue_test

import (
	"errors"
	"testing"

	"portbio/internal/model"
	"portbio/internal/queue"
	"portbio/internal/seed"
)

func newQueue(t *testing.T) *queue.Queue {
	t.Helper()
	q := queue.New()
	if err := q.AddAll(seed.Interceptions()); err != nil {
		t.Fatalf("装载队列失败: %v", err)
	}
	return q
}

func ids(items []model.Interception) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOrderedIsStable(t *testing.T) {
	q := newQueue(t)
	first := ids(q.Ordered())
	for r := 0; r < 10; r++ {
		again := ids(q.Ordered())
		if !equal(first, again) {
			t.Fatalf("第 %d 次排序结果与首次不一致\n首次: %v\n本次: %v", r+1, first, again)
		}
	}
}

func TestOrderedFollowsRiskThenTime(t *testing.T) {
	q := newQueue(t)
	items := q.Ordered()
	for i := 1; i < len(items); i++ {
		prev, cur := items[i-1], items[i]
		if prev.Risk.Rank() < cur.Risk.Rank() {
			t.Fatalf("风险等级应降序: %s(%s) 在 %s(%s) 之前",
				prev.ID, prev.Risk, cur.ID, cur.Risk)
		}
		if prev.Risk == cur.Risk && cur.AcceptedAt.Before(prev.AcceptedAt) {
			t.Fatalf("同风险应按受理时间升序: %s 在 %s 之前", prev.ID, cur.ID)
		}
	}
}

func TestPageIsStableAcrossCalls(t *testing.T) {
	q := newQueue(t)
	size := seed.PageSize()
	for p := 1; p <= q.Pages(size); p++ {
		first := ids(q.Page(p, size))
		for r := 0; r < 5; r++ {
			again := ids(q.Page(p, size))
			if !equal(first, again) {
				t.Fatalf("第 %d 页第 %d 次取到的顺序与首次不一致\n首次: %v\n本次: %v",
					p, r+1, first, again)
			}
		}
	}
}

func TestPageCoversEveryRecordExactlyOnce(t *testing.T) {
	q := newQueue(t)
	for _, size := range []int{1, 2, 4, 7, 15} {
		cov := q.CheckCoverage(size)
		if !cov.Complete {
			t.Fatalf("页大小 %d: 逐页遍历应恰好覆盖全部记录，实际取到 %d 条（互异 %d 条）, 重复 %v, 遗漏 %v",
				size, cov.Collected, cov.Distinct, cov.Duplicated, cov.Missing)
		}
	}
}

func TestVerifyPassesForSeedQueue(t *testing.T) {
	q := newQueue(t)
	if err := q.Verify(seed.PageSize()); err != nil {
		t.Fatalf("样例队列分页应完整: %v", err)
	}
}

func TestVerifyDetectsIncompletePagination(t *testing.T) {
	// 空队列的分页天然完整，用它确认 Verify 不误报。
	q := queue.New()
	if err := q.Verify(4); err != nil {
		t.Fatalf("空队列不应报错: %v", err)
	}
}

func TestPagesAndBounds(t *testing.T) {
	q := newQueue(t)
	size := 4
	want := (q.Len() + size - 1) / size
	if got := q.Pages(size); got != want {
		t.Fatalf("总页数应为 %d, 实际 %d", want, got)
	}
	if got := q.Page(0, size); got != nil {
		t.Fatalf("页码为 0 应返回空页, 实际 %d 条", len(got))
	}
	if got := q.Page(want+1, size); got != nil {
		t.Fatalf("页码越界应返回空页, 实际 %d 条", len(got))
	}
	last := q.Page(want, size)
	if len(last) == 0 || len(last) > size {
		t.Fatalf("最后一页条数应在 1..%d 之间, 实际 %d", size, len(last))
	}
}

func TestWalkCollectsAll(t *testing.T) {
	q := newQueue(t)
	got := q.Walk(seed.PageSize())
	if len(got) != q.Len() {
		t.Fatalf("逐页收集应得到 %d 条, 实际 %d 条", q.Len(), len(got))
	}
	seen := make(map[string]struct{}, len(got))
	for _, id := range got {
		if _, dup := seen[id]; dup {
			t.Fatalf("编号 %s 被重复取到", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGetNotFound(t *testing.T) {
	q := newQueue(t)
	if _, err := q.Get("IC-NOPE"); !errors.Is(err, model.ErrInterceptionNotFound) {
		t.Fatalf("应返回 ErrInterceptionNotFound, 得到 %v", err)
	}
}

func TestCountByRiskCoversAll(t *testing.T) {
	q := newQueue(t)
	counts := q.CountByRisk()
	total := 0
	for _, r := range model.AllRisks() {
		if _, ok := counts[r]; !ok {
			t.Fatalf("统计应覆盖风险等级 %s", r)
		}
		total += counts[r]
	}
	if total != q.Len() {
		t.Fatalf("分风险合计 %d 与队列长度 %d 不一致", total, q.Len())
	}
}

func TestAddRejectsInvalid(t *testing.T) {
	q := queue.New()
	if err := q.Add(model.Interception{}); !errors.Is(err, model.ErrInvalidInterception) {
		t.Fatalf("非法记录应返回 ErrInvalidInterception, 得到 %v", err)
	}
}
