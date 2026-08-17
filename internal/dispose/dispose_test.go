package dispose_test

import (
	"errors"
	"testing"

	"portbio/internal/dispose"
	"portbio/internal/model"
	"portbio/internal/seed"
)

func newService(t *testing.T) *dispose.Service {
	t.Helper()
	svc := dispose.NewService(seed.PortCode())
	mk := func(note string) dispose.Handler {
		return func(it model.Interception) (dispose.Disposal, error) {
			return dispose.Disposal{
				InterceptionID: it.ID, Measure: it.Measure,
				Handled: it.Quantity, Note: note,
			}, nil
		}
	}
	for _, m := range []model.Measure{
		model.MeasureRelease, model.MeasureDisinfect,
		model.MeasureReturn, model.MeasureDestroy,
	} {
		if err := svc.Register(m, mk("已处置")); err != nil {
			t.Fatalf("注册处置器失败: %v", err)
		}
	}
	return svc
}

func labHoldRecord() model.Interception {
	return model.Interception{
		ID: "IC-TEST", PortCode: seed.PortCode(), SpeciesCode: "SP-001",
		Channel: model.ChannelMail, Risk: model.RiskQuarantine,
		Measure: seed.UnregisteredMeasure(), Quantity: 3, AcceptedAt: seed.Now(),
	}
}

func TestDisposeUnregisteredMeasureReturnsSentinel(t *testing.T) {
	svc := newService(t)
	if svc.Supports(seed.UnregisteredMeasure()) {
		t.Fatalf("措施 %s 在本口岸不应有处置器", seed.UnregisteredMeasure())
	}
	_, err := svc.Dispose(labHoldRecord())
	if !errors.Is(err, model.ErrHandlerMissing) {
		t.Fatalf("未注册处置器应返回 ErrHandlerMissing, 得到 %v", err)
	}
}

func TestDisposeUnregisteredAcrossRecords(t *testing.T) {
	svc := newService(t)
	for _, id := range seed.LabHoldInterceptionIDs() {
		var target model.Interception
		for _, it := range seed.Interceptions() {
			if it.ID == id {
				target = it
			}
		}
		if _, err := svc.Dispose(target); !errors.Is(err, model.ErrHandlerMissing) {
			t.Fatalf("记录 %s 应返回 ErrHandlerMissing, 得到 %v", id, err)
		}
	}
}

func TestDisposeRegisteredMeasureSucceeds(t *testing.T) {
	svc := newService(t)
	items := seed.Interceptions()
	handled := 0
	for _, it := range items {
		if it.Measure == seed.UnregisteredMeasure() {
			continue
		}
		d, err := svc.Dispose(it)
		if err != nil {
			t.Fatalf("记录 %s 处置应当成功: %v", it.ID, err)
		}
		if d.Handled != it.Quantity {
			t.Fatalf("记录 %s 处置数量应为 %d, 实际 %d", it.ID, it.Quantity, d.Handled)
		}
		handled++
	}
	if handled == 0 {
		t.Fatal("样例数据应包含可处置的记录")
	}
	if len(svc.Done()) != handled {
		t.Fatalf("处置记录应为 %d 条, 实际 %d 条", handled, len(svc.Done()))
	}
}

func TestDisposeAllSkipsUnregistered(t *testing.T) {
	svc := newService(t)
	items := seed.Interceptions()
	done, skipped, err := svc.DisposeAll(items)
	if err != nil {
		t.Fatalf("批量处置不应中断: %v", err)
	}
	if len(skipped) != len(seed.LabHoldInterceptionIDs()) {
		t.Fatalf("应跳过 %d 条, 实际 %d 条",
			len(seed.LabHoldInterceptionIDs()), len(skipped))
	}
	if len(done)+len(skipped) != len(items) {
		t.Fatalf("处置 %d 条 + 跳过 %d 条应等于提交 %d 条",
			len(done), len(skipped), len(items))
	}
}

func TestRegisterRejectsNilHandler(t *testing.T) {
	svc := dispose.NewService(seed.PortCode())
	if err := svc.Register(model.MeasureDestroy, nil); !errors.Is(err, model.ErrHandlerMissing) {
		t.Fatalf("空处置器应返回 ErrHandlerMissing, 得到 %v", err)
	}
	if err := svc.Register(model.Measure("x"), func(model.Interception) (dispose.Disposal, error) {
		return dispose.Disposal{}, nil
	}); !errors.Is(err, model.ErrUnknownMeasure) {
		t.Fatalf("未知措施应返回 ErrUnknownMeasure, 得到 %v", err)
	}
}

func TestDisposeRejectsInvalidRecord(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Dispose(model.Interception{}); !errors.Is(err, model.ErrInvalidInterception) {
		t.Fatalf("非法记录应返回 ErrInvalidInterception, 得到 %v", err)
	}
}

func TestRegisteredListsOnlyRegistered(t *testing.T) {
	svc := newService(t)
	got := svc.Registered()
	if len(got) != 4 {
		t.Fatalf("应有 4 个已注册措施, 实际 %v", got)
	}
	for _, m := range got {
		if m == string(seed.UnregisteredMeasure()) {
			t.Fatalf("措施 %s 不应出现在已注册列表中", m)
		}
	}
}

func TestHandledQuantityMatchesDone(t *testing.T) {
	svc := newService(t)
	if _, _, err := svc.DisposeAll(seed.Interceptions()); err != nil {
		t.Fatalf("批量处置失败: %v", err)
	}
	want := 0
	for _, d := range svc.Done() {
		want += d.Handled
	}
	if got := svc.HandledQuantity(); got != want {
		t.Fatalf("已处置数量应为 %d, 实际 %d", want, got)
	}
}
