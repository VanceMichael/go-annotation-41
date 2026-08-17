package report_test

import (
	"testing"

	"portbio/internal/dispose"
	"portbio/internal/model"
	"portbio/internal/queue"
	"portbio/internal/report"
	"portbio/internal/seed"
	"portbio/internal/species"
)

func catalog(t *testing.T) *species.Catalog {
	t.Helper()
	c := species.NewCatalog()
	if err := c.AddAll(seed.SpeciesList()); err != nil {
		t.Fatalf("装载名录失败: %v", err)
	}
	return c
}

func newQueue(t *testing.T) *queue.Queue {
	t.Helper()
	q := queue.New()
	if err := q.AddAll(seed.Interceptions()); err != nil {
		t.Fatalf("装载队列失败: %v", err)
	}
	return q
}

func newDispose(t *testing.T) *dispose.Service {
	t.Helper()
	svc := dispose.NewService(seed.PortCode())
	h := func(it model.Interception) (dispose.Disposal, error) {
		return dispose.Disposal{
			InterceptionID: it.ID, Measure: it.Measure, Handled: it.Quantity, Note: "已处置",
		}, nil
	}
	for _, m := range []model.Measure{
		model.MeasureRelease, model.MeasureDisinfect,
		model.MeasureReturn, model.MeasureDestroy,
	} {
		if err := svc.Register(m, h); err != nil {
			t.Fatalf("注册处置器失败: %v", err)
		}
	}
	return svc
}

func TestCatalogSummary(t *testing.T) {
	s := report.Catalog(catalog(t))
	if s.Species != len(seed.SpeciesList()) {
		t.Fatalf("名录条数应为 %d, 实际 %d", len(seed.SpeciesList()), s.Species)
	}
	if s.Quarantine != 2 {
		t.Fatalf("检疫名单应为 2 条, 实际 %d", s.Quarantine)
	}
	total := 0
	for _, r := range model.AllRisks() {
		total += s.ByRisk[string(r)]
	}
	if total != s.Species {
		t.Fatalf("分等级合计 %d 与名录条数 %d 不一致", total, s.Species)
	}
	if report.Describe(s) == "" {
		t.Fatal("描述不应为空")
	}
}

func TestQueueReportCoverageComplete(t *testing.T) {
	q := newQueue(t)
	sum := report.Queue(q, seed.PageSize())
	if sum.Total != q.Len() {
		t.Fatalf("队列长度应为 %d, 实际 %d", q.Len(), sum.Total)
	}
	if !sum.Coverage.Complete {
		t.Fatalf("逐页遍历应完整: 取到 %d 条（互异 %d 条）, 重复 %v, 遗漏 %v",
			sum.Coverage.Collected, sum.Coverage.Distinct,
			sum.Coverage.Duplicated, sum.Coverage.Missing)
	}
	if sum.Pages != q.Pages(seed.PageSize()) {
		t.Fatalf("页数不符: %d", sum.Pages)
	}
}

func TestDisposalReportSkipsUnregistered(t *testing.T) {
	svc := newDispose(t)
	items := seed.Interceptions()
	rows, sum, err := report.Disposal(svc, items)
	if err != nil {
		t.Fatalf("处置报表不应报错: %v", err)
	}
	if len(rows) != len(model.AllMeasures()) {
		t.Fatalf("报表应覆盖全部措施: %d", len(rows))
	}
	if sum.Skipped != len(seed.LabHoldInterceptionIDs()) {
		t.Fatalf("应跳过 %d 条, 实际 %d 条",
			len(seed.LabHoldInterceptionIDs()), sum.Skipped)
	}
	if sum.Handled+sum.Skipped != len(items) {
		t.Fatalf("处置 %d + 跳过 %d 应等于 %d", sum.Handled, sum.Skipped, len(items))
	}
	for _, r := range rows {
		if r.Measure == string(seed.UnregisteredMeasure()) && r.Registered {
			t.Fatalf("措施 %s 不应标记为已注册", r.Measure)
		}
	}
}

func TestDisposalRowCountsMatchItems(t *testing.T) {
	svc := newDispose(t)
	items := seed.Interceptions()
	rows, _, err := report.Disposal(svc, items)
	if err != nil {
		t.Fatalf("处置报表不应报错: %v", err)
	}
	total := 0
	for _, r := range rows {
		total += r.Count
	}
	if total != len(items) {
		t.Fatalf("各措施记录数合计 %d 与提交 %d 不一致", total, len(items))
	}
}
