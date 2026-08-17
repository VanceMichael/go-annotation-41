// Package seed 提供内置样例数据，仅用于本地演练。
package seed

import (
	"fmt"
	"time"

	"portbio/internal/model"
)

func ts(day, hour, minute int) time.Time {
	return time.Date(2026, 8, day, hour, minute, 0, 0, time.UTC)
}

// Now 返回样例数据使用的固定当前时间。
func Now() time.Time {
	return ts(16, 9, 0)
}

// PortCode 返回演练所用口岸代码。
func PortCode() string {
	return "TJ-PORT"
}

// SpeciesList 返回样例物种名录。
func SpeciesList() []model.Species {
	return []model.Species{
		{Code: "SP-001", NameZH: "佛氏圆尾鳉", NameLatin: "Nothobranchius foerschi",
			Risk: model.RiskQuarantine, Category: "鱼类", Quarantine: true},
		{Code: "SP-002", NameZH: "红火蚁", NameLatin: "Solenopsis invicta",
			Risk: model.RiskQuarantine, Category: "昆虫", Quarantine: true},
		{Code: "SP-003", NameZH: "非洲大蜗牛", NameLatin: "Lissachatina fulica",
			Risk: model.RiskHigh, Category: "软体动物"},
		{Code: "SP-004", NameZH: "水葫芦", NameLatin: "Eichhornia crassipes",
			Risk: model.RiskHigh, Category: "植物"},
		{Code: "SP-005", NameZH: "巴西龟", NameLatin: "Trachemys scripta elegans",
			Risk: model.RiskMedium, Category: "爬行动物"},
		{Code: "SP-006", NameZH: "多肉植物种子", NameLatin: "Crassulaceae spp.",
			Risk: model.RiskMedium, Category: "植物"},
		{Code: "SP-007", NameZH: "观赏石斛", NameLatin: "Dendrobium spp.",
			Risk: model.RiskLow, Category: "植物"},
	}
}

// UnregisteredMeasure 返回本口岸未注册处置器的措施。
//
// 送实验室隔离检测需上级授权，本口岸未配置对应处置器。
func UnregisteredMeasure() model.Measure {
	return model.MeasureLabHold
}

// Interceptions 返回样例截获记录。
//
// 其中两条的处置措施是「送实验室隔离检测」，本口岸未注册该措施的处置器。
func Interceptions() []model.Interception {
	type row struct {
		id      string
		species string
		ch      model.Channel
		risk    model.Risk
		measure model.Measure
		qty     int
		day     int
		hour    int
		minute  int
	}
	rows := []row{
		{"IC-001", "SP-001", model.ChannelMail, model.RiskQuarantine, model.MeasureDestroy, 12, 14, 9, 10},
		{"IC-002", "SP-002", model.ChannelCargo, model.RiskQuarantine, model.MeasureDestroy, 400, 14, 11, 5},
		{"IC-003", "SP-003", model.ChannelBaggage, model.RiskHigh, model.MeasureDestroy, 6, 14, 13, 20},
		{"IC-004", "SP-004", model.ChannelCargo, model.RiskHigh, model.MeasureReturn, 30, 15, 8, 40},
		{"IC-005", "SP-005", model.ChannelExpress, model.RiskMedium, model.MeasureReturn, 3, 15, 9, 15},
		{"IC-006", "SP-006", model.ChannelMail, model.RiskMedium, model.MeasureDisinfect, 120, 15, 10, 30},
		{"IC-007", "SP-007", model.ChannelBaggage, model.RiskLow, model.MeasureRelease, 2, 15, 11, 45},
		{"IC-008", "SP-003", model.ChannelExpress, model.RiskHigh, model.MeasureDisinfect, 8, 15, 14, 0},
		{"IC-009", "SP-001", model.ChannelExpress, model.RiskQuarantine, model.MeasureLabHold, 5, 16, 8, 5},
		{"IC-010", "SP-002", model.ChannelBaggage, model.RiskQuarantine, model.MeasureLabHold, 60, 16, 8, 25},
		{"IC-011", "SP-005", model.ChannelMail, model.RiskMedium, model.MeasureRelease, 1, 16, 8, 50},
		{"IC-012", "SP-006", model.ChannelCargo, model.RiskMedium, model.MeasureDisinfect, 900, 16, 9, 10},
		{"IC-013", "SP-004", model.ChannelCargo, model.RiskHigh, model.MeasureReturn, 45, 16, 9, 30},
		{"IC-014", "SP-007", model.ChannelMail, model.RiskLow, model.MeasureRelease, 4, 16, 9, 55},
		{"IC-015", "SP-003", model.ChannelBaggage, model.RiskHigh, model.MeasureDestroy, 11, 16, 10, 20},
	}
	out := make([]model.Interception, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.Interception{
			ID: r.id, PortCode: PortCode(), SpeciesCode: r.species,
			Channel: r.ch, Risk: r.risk, Measure: r.measure, Quantity: r.qty,
			AcceptedAt: ts(r.day, r.hour, r.minute),
		})
	}
	return out
}

// LabHoldInterceptionIDs 返回处置措施为「送实验室隔离检测」的截获编号。
func LabHoldInterceptionIDs() []string {
	out := make([]string, 0, 2)
	for _, it := range Interceptions() {
		if it.Measure == UnregisteredMeasure() {
			out = append(out, it.ID)
		}
	}
	return out
}

// Samples 返回样例送检样本。
//
// 其中含加急样本，用于演练快速通道。
func Samples(n int) []model.Sample {
	if n <= 0 {
		n = 12
	}
	out := make([]model.Sample, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, model.Sample{
			ID:             fmt.Sprintf("SM-%03d", i),
			InterceptionID: fmt.Sprintf("IC-%03d", (i-1)%15+1),
			Priority:       i%3 == 0,
			AssayMS:        2,
		})
	}
	return out
}

// PrioritySampleCount 返回样例样本中加急样本的数量。
func PrioritySampleCount(n int) int {
	c := 0
	for _, s := range Samples(n) {
		if s.Priority {
			c++
		}
	}
	return c
}

// PageSize 返回查验队列演练所用的默认页大小。
func PageSize() int {
	return 4
}
