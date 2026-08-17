package sweep_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"portbio/internal/model"
	"portbio/internal/seed"
	"portbio/internal/sweep"
)

func TestSweepCompletesWithAmpleBudget(t *testing.T) {
	sw := sweep.New(0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	items := seed.Interceptions()
	n, err := sw.Sweep(ctx, items)
	if err != nil {
		t.Fatalf("超时充足时不应中止: %v", err)
	}
	if n != len(items) {
		t.Fatalf("应复核完 %d 条, 实际 %d 条", len(items), n)
	}
}

func TestSweepStopsWhenContextExpires(t *testing.T) {
	perItem := 20 * time.Millisecond
	timeout := 100 * time.Millisecond
	sw := sweep.New(perItem)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	items := seed.Interceptions()
	start := time.Now()
	n, err := sw.Sweep(ctx, items)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("超时后必须中止并上报，实际复核完 %d 条且返回 nil", n)
	}
	if n >= len(items) {
		t.Fatalf("超时后不应把剩余记录跑完: 复核 %d/%d 条", n, len(items))
	}
	// 15 条 × 20ms = 300ms，远超 100ms 预算；及时中止应在 200ms 内返回。
	if elapsed > 200*time.Millisecond {
		t.Fatalf("应在超时附近返回, 实际耗时 %v（复核 %d 条）", elapsed, n)
	}
}

func TestSweepSurfacesContextError(t *testing.T) {
	sw := sweep.New(20 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	_, err := sw.Sweep(ctx, seed.Interceptions())
	if !errors.Is(err, model.ErrSweepAborted) {
		t.Fatalf("应返回 ErrSweepAborted, 得到 %v", err)
	}
}

func TestSweepHonoursCancel(t *testing.T) {
	sw := sweep.New(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	items := seed.Interceptions()
	n, err := sw.Sweep(ctx, items)
	if err == nil {
		t.Fatalf("被取消后必须中止，实际复核完 %d 条且返回 nil", n)
	}
	if n >= len(items) {
		t.Fatalf("被取消后不应跑完全部记录: %d/%d", n, len(items))
	}
}

func TestSweepAlreadyCancelledContext(t *testing.T) {
	sw := sweep.New(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := sw.Sweep(ctx, seed.Interceptions())
	if err == nil {
		t.Fatal("已取消的 context 应立即中止")
	}
	if n != 0 {
		t.Fatalf("已取消时不应复核任何记录, 实际 %d 条", n)
	}
}

func TestSweepBatchReportsAbort(t *testing.T) {
	sw := sweep.New(20 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	rep := sw.SweepBatch(ctx, seed.Interceptions())
	if !rep.Aborted {
		t.Fatalf("摘要应标记为已中止: %+v", rep)
	}
	if rep.Checked >= rep.Submitted {
		t.Fatalf("中止时复核条数应少于提交条数: %d/%d", rep.Checked, rep.Submitted)
	}
	if rep.Message == "" {
		t.Fatal("中止时应带中止原因")
	}
}

func TestSweepFlagsRiskyRecords(t *testing.T) {
	sw := sweep.New(0)
	items := seed.Interceptions()
	if _, err := sw.Sweep(context.Background(), items); err != nil {
		t.Fatalf("复核失败: %v", err)
	}
	flagged := 0
	for _, f := range sw.Findings() {
		if f.Flagged {
			flagged++
		}
	}
	if flagged == 0 {
		t.Fatal("样例数据应存在需人工复核的记录")
	}
}

func TestFindingsCountMatchesChecked(t *testing.T) {
	sw := sweep.New(0)
	items := seed.Interceptions()
	n, err := sw.Sweep(context.Background(), items)
	if err != nil {
		t.Fatalf("复核失败: %v", err)
	}
	if len(sw.Findings()) != n || sw.Checked() != n {
		t.Fatalf("结论数 %d / 计数 %d 应等于复核条数 %d",
			len(sw.Findings()), sw.Checked(), n)
	}
}
