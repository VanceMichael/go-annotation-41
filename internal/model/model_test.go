package model_test

import (
	"errors"
	"testing"
	"time"

	"portbio/internal/model"
)

func TestParseRiskRoundTrip(t *testing.T) {
	for _, r := range model.AllRisks() {
		got, err := model.ParseRisk(string(r))
		if err != nil || got != r || got.DisplayName() == "" {
			t.Fatalf("风险等级 %s 解析异常: %s %v", r, got, err)
		}
	}
	if _, err := model.ParseRisk("extreme"); !errors.Is(err, model.ErrUnknownRisk) {
		t.Fatalf("未知等级应返回 ErrUnknownRisk, 得到 %v", err)
	}
}

func TestRiskRankIsMonotonic(t *testing.T) {
	rs := model.AllRisks()
	for i := 1; i < len(rs); i++ {
		if rs[i].Rank() <= rs[i-1].Rank() {
			t.Fatalf("风险等级应递增: %s(%d) <= %s(%d)",
				rs[i], rs[i].Rank(), rs[i-1], rs[i-1].Rank())
		}
	}
}

func TestParseMeasureRoundTrip(t *testing.T) {
	for _, m := range model.AllMeasures() {
		got, err := model.ParseMeasure(string(m))
		if err != nil || got != m || got.DisplayName() == "" {
			t.Fatalf("措施 %s 解析异常: %s %v", m, got, err)
		}
	}
	if _, err := model.ParseMeasure("burn"); !errors.Is(err, model.ErrUnknownMeasure) {
		t.Fatalf("未知措施应返回 ErrUnknownMeasure, 得到 %v", err)
	}
}

func TestMeasurePermittedFor(t *testing.T) {
	if model.MeasureRelease.PermittedFor(model.RiskQuarantine) {
		t.Fatal("检疫性有害生物不得放行")
	}
	if model.MeasureRelease.PermittedFor(model.RiskHigh) {
		t.Fatal("高风险不得直接放行")
	}
	if !model.MeasureRelease.PermittedFor(model.RiskMedium) {
		t.Fatal("中风险应可放行")
	}
	if model.MeasureDisinfect.PermittedFor(model.RiskQuarantine) {
		t.Fatal("检疫性有害生物不得仅消杀后放行")
	}
	if !model.MeasureDestroy.PermittedFor(model.RiskQuarantine) {
		t.Fatal("销毁应适用于任何等级")
	}
}

func TestParseChannelRoundTrip(t *testing.T) {
	for _, c := range model.AllChannels() {
		got, err := model.ParseChannel(string(c))
		if err != nil || got != c || got.DisplayName() == "" {
			t.Fatalf("渠道 %s 解析异常: %s %v", c, got, err)
		}
	}
	if _, err := model.ParseChannel("rail"); !errors.Is(err, model.ErrUnknownChannel) {
		t.Fatalf("未知渠道应返回 ErrUnknownChannel, 得到 %v", err)
	}
}

func TestSpeciesValidate(t *testing.T) {
	ok := model.Species{Code: "SP-1", NameZH: "甲", NameLatin: "A", Risk: model.RiskHigh}
	if err := ok.Validate(); err != nil {
		t.Fatalf("合法名录应通过: %v", err)
	}
	bad := ok
	bad.Quarantine = true
	if err := bad.Validate(); !errors.Is(err, model.ErrInvalidSpecies) {
		t.Fatalf("检疫名单与等级不一致应返回 ErrInvalidSpecies, 得到 %v", err)
	}
}

func interceptionFixture() model.Interception {
	return model.Interception{
		ID: "IC-1", PortCode: "P", SpeciesCode: "SP-1",
		Channel: model.ChannelMail, Risk: model.RiskHigh,
		Measure: model.MeasureDestroy, Quantity: 1,
		AcceptedAt: time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC),
	}
}

func TestInterceptionValidate(t *testing.T) {
	if err := interceptionFixture().Validate(); err != nil {
		t.Fatalf("合法记录应通过: %v", err)
	}
	for _, mut := range []func(*model.Interception){
		func(i *model.Interception) { i.Quantity = 0 },
		func(i *model.Interception) { i.PortCode = "" },
		func(i *model.Interception) { i.SpeciesCode = "" },
		func(i *model.Interception) { i.AcceptedAt = time.Time{} },
	} {
		bad := interceptionFixture()
		mut(&bad)
		if err := bad.Validate(); !errors.Is(err, model.ErrInvalidInterception) {
			t.Fatalf("非法记录应返回 ErrInvalidInterception, 得到 %v", err)
		}
	}
}

func TestSampleValidate(t *testing.T) {
	ok := model.Sample{ID: "SM-1", InterceptionID: "IC-1", AssayMS: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("合法样本应通过: %v", err)
	}
	bad := ok
	bad.AssayMS = -1
	if err := bad.Validate(); !errors.Is(err, model.ErrInvalidSample) {
		t.Fatalf("负耗时应返回 ErrInvalidSample, 得到 %v", err)
	}
}

func TestSortInterceptionsOrder(t *testing.T) {
	base := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)
	items := []model.Interception{
		{ID: "C", Risk: model.RiskMedium, AcceptedAt: base},
		{ID: "A", Risk: model.RiskQuarantine, AcceptedAt: base.Add(time.Hour)},
		{ID: "B", Risk: model.RiskQuarantine, AcceptedAt: base},
	}
	model.SortInterceptions(items)
	if items[0].ID != "B" || items[1].ID != "A" || items[2].ID != "C" {
		t.Fatalf("排序结果不符: %v", items)
	}
}
