// Package report 汇总名录、查验队列与处置报表。
package report

import (
	"fmt"

	"portbio/internal/dispose"
	"portbio/internal/model"
	"portbio/internal/queue"
	"portbio/internal/species"
)

// CatalogSummary 是物种名录汇总。
type CatalogSummary struct {
	Species int            `json:"species"`
	ByRisk  map[string]int `json:"by_risk"`
	// Quarantine 是列入检疫性有害生物名单的物种数。
	Quarantine int `json:"quarantine"`
}

// Catalog 生成名录报表。
func Catalog(c *species.Catalog) CatalogSummary {
	counts := c.CountByRisk()
	byRisk := make(map[string]int, len(counts))
	for _, r := range model.AllRisks() {
		byRisk[string(r)] = counts[r]
	}
	return CatalogSummary{
		Species:    len(c.List()),
		ByRisk:     byRisk,
		Quarantine: len(c.QuarantineCodes()),
	}
}

// QueueSummary 是查验队列汇总。
type QueueSummary struct {
	Total  int            `json:"total"`
	ByRisk map[string]int `json:"by_risk"`
	// PageSize 是分页核对所用的页大小。
	PageSize int `json:"page_size"`
	Pages    int `json:"pages"`
	// Coverage 是逐页遍历的覆盖情况。
	Coverage queue.Coverage `json:"coverage"`
}

// Queue 生成查验队列报表。
func Queue(q *queue.Queue, pageSize int) QueueSummary {
	counts := q.CountByRisk()
	byRisk := make(map[string]int, len(counts))
	for _, r := range model.AllRisks() {
		byRisk[string(r)] = counts[r]
	}
	return QueueSummary{
		Total:    q.Len(),
		ByRisk:   byRisk,
		PageSize: pageSize,
		Pages:    q.Pages(pageSize),
		Coverage: q.CheckCoverage(pageSize),
	}
}

// DisposalRow 是处置报表中的一行。
type DisposalRow struct {
	Measure string `json:"measure"`
	// Registered 报告本口岸是否注册了该措施的处置器。
	Registered bool `json:"registered"`
	// Count 是该措施在队列中的记录条数。
	Count int `json:"count"`
	// Quantity 是该措施涉及的截获总数量。
	Quantity int `json:"quantity"`
}

// DisposalSummary 是处置报表汇总。
type DisposalSummary struct {
	// Handled 是已完成处置的记录条数。
	Handled int `json:"handled"`
	// Skipped 是因本口岸未注册处置器而跳过的记录条数。
	Skipped int `json:"skipped"`
	// SkippedIDs 是被跳过的记录编号。
	SkippedIDs []string `json:"skipped_ids"`
	// HandledQuantity 是已处置的总数量。
	HandledQuantity int `json:"handled_quantity"`
}

// Disposal 生成处置报表。
//
// 未注册处置器的措施应当被跳过并计入 skipped，不影响其余记录的处置。
func Disposal(svc *dispose.Service, items []model.Interception) ([]DisposalRow, DisposalSummary, error) {
	counts := make(map[model.Measure]int)
	qty := make(map[model.Measure]int)
	for _, it := range items {
		counts[it.Measure]++
		qty[it.Measure] += it.Quantity
	}

	rows := make([]DisposalRow, 0, len(model.AllMeasures()))
	for _, m := range model.AllMeasures() {
		rows = append(rows, DisposalRow{
			Measure:    string(m),
			Registered: svc.Supports(m),
			Count:      counts[m],
			Quantity:   qty[m],
		})
	}

	done, skipped, err := svc.DisposeAll(items)
	if err != nil {
		return rows, DisposalSummary{}, err
	}
	sum := DisposalSummary{
		Handled:         len(done),
		Skipped:         len(skipped),
		SkippedIDs:      skipped,
		HandledQuantity: svc.HandledQuantity(),
	}
	return rows, sum, nil
}

// Describe 返回名录汇总的单行描述。
func Describe(s CatalogSummary) string {
	return fmt.Sprintf("名录%d 条（检疫名单%d 条）", s.Species, s.Quarantine)
}
