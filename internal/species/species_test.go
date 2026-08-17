package species_test

import (
	"errors"
	"testing"

	"portbio/internal/model"
	"portbio/internal/seed"
	"portbio/internal/species"
)

func newCatalog(t *testing.T) *species.Catalog {
	t.Helper()
	c := species.NewCatalog()
	if err := c.AddAll(seed.SpeciesList()); err != nil {
		t.Fatalf("装载名录失败: %v", err)
	}
	return c
}

func TestGetAndList(t *testing.T) {
	c := newCatalog(t)
	if len(c.List()) != len(seed.SpeciesList()) {
		t.Fatalf("名录条数应为 %d, 实际 %d", len(seed.SpeciesList()), len(c.List()))
	}
	s, err := c.Get("SP-001")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if s.Risk != model.RiskQuarantine {
		t.Fatalf("SP-001 应为检疫性有害生物, 实际 %s", s.Risk)
	}
	if _, err := c.Get("SP-NOPE"); !errors.Is(err, model.ErrSpeciesNotFound) {
		t.Fatalf("应返回 ErrSpeciesNotFound, 得到 %v", err)
	}
}

func TestQuarantineCodesSorted(t *testing.T) {
	c := newCatalog(t)
	got := c.QuarantineCodes()
	if len(got) != 2 {
		t.Fatalf("检疫名单应有 2 条, 实际 %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("结果应升序: %v", got)
		}
	}
}

func TestRiskOf(t *testing.T) {
	c := newCatalog(t)
	r, err := c.RiskOf("SP-003")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if r != model.RiskHigh {
		t.Fatalf("SP-003 应为高风险, 实际 %s", r)
	}
	if _, err := c.RiskOf("SP-NOPE"); !errors.Is(err, model.ErrSpeciesNotFound) {
		t.Fatalf("应返回 ErrSpeciesNotFound, 得到 %v", err)
	}
}

func TestCountByRiskCoversAll(t *testing.T) {
	c := newCatalog(t)
	counts := c.CountByRisk()
	total := 0
	for _, r := range model.AllRisks() {
		if _, ok := counts[r]; !ok {
			t.Fatalf("统计应覆盖等级 %s", r)
		}
		total += counts[r]
	}
	if total != len(seed.SpeciesList()) {
		t.Fatalf("分等级合计 %d 与名录条数 %d 不一致", total, len(seed.SpeciesList()))
	}
}

func TestCheckMeasure(t *testing.T) {
	c := newCatalog(t)
	bad := model.Interception{
		ID: "IC-X", PortCode: "P", SpeciesCode: "SP-001",
		Channel: model.ChannelMail, Risk: model.RiskQuarantine,
		Measure: model.MeasureRelease, Quantity: 1, AcceptedAt: seed.Now(),
	}
	if err := c.CheckMeasure(bad); !errors.Is(err, model.ErrMeasureNotPermitted) {
		t.Fatalf("检疫性有害生物放行应返回 ErrMeasureNotPermitted, 得到 %v", err)
	}
	good := bad
	good.Measure = model.MeasureDestroy
	if err := c.CheckMeasure(good); err != nil {
		t.Fatalf("销毁应被允许: %v", err)
	}
}

func TestAddRejectsInvalid(t *testing.T) {
	c := species.NewCatalog()
	if err := c.Add(model.Species{}); !errors.Is(err, model.ErrInvalidSpecies) {
		t.Fatalf("空条目应返回 ErrInvalidSpecies, 得到 %v", err)
	}
}

func TestDescribeNotEmpty(t *testing.T) {
	c := newCatalog(t)
	s, err := c.Get("SP-002")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if species.Describe(s) == "" {
		t.Fatal("描述不应为空")
	}
}
