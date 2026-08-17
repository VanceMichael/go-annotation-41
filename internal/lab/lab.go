// Package lab 负责实验室样本的并发检测编排。
package lab

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"portbio/internal/model"
)

// Result 是一份样本的检测结果。
type Result struct {
	SampleID string `json:"sample_id"`
	// Positive 报告检出结果是否为阳性。
	Positive bool `json:"positive"`
	// Priority 报告该样本是否为加急样本。
	Priority bool `json:"priority"`
	// ElapsedMS 是该样本的检测耗时毫秒数。
	ElapsedMS int64 `json:"elapsed_ms"`
}

// Lab 是实验室检测编排器。
type Lab struct {
	// Workers 是并发检测的工位数上限。
	Workers int
}

// New 构造实验室检测编排器。
func New(workers int) *Lab {
	if workers <= 0 {
		workers = 4
	}
	return &Lab{Workers: workers}
}

// assay 执行单份样本的检测。
func assay(s model.Sample) Result {
	start := time.Now()
	if s.AssayMS > 0 {
		time.Sleep(time.Duration(s.AssayMS) * time.Millisecond)
	}
	// 演练用判定：样本编号末位为偶数视为阳性。
	positive := false
	if n := len(s.ID); n > 0 {
		c := s.ID[n-1]
		positive = c >= '0' && c <= '9' && (c-'0')%2 == 0
	}
	return Result{
		SampleID:  s.ID,
		Positive:  positive,
		Priority:  s.Priority,
		ElapsedMS: time.Since(start).Milliseconds(),
	}
}

// RunBatch 并发检测一批样本。
//
// 加急样本走快速通道，但无论走哪条通道，每份样本都必须产出一条结果：
// 返回的结果条数必须等于送检样本数。
func (l *Lab) RunBatch(samples []model.Sample) ([]Result, error) {
	for _, s := range samples {
		if err := s.Validate(); err != nil {
			return nil, err
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	out := make([]Result, 0, len(samples))
	sem := make(chan struct{}, l.Workers)

	for _, s := range samples {
		wg.Add(1)
		go func(s model.Sample) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if s.Priority {
				// 加急样本走快速通道，检测完立刻登记结果。
				r := assay(s)
				mu.Lock()
				out = append(out, r)
				mu.Unlock()
				return
			}

			r := assay(s)
			mu.Lock()
			out = append(out, r)
			mu.Unlock()
		}(s)
	}
	wg.Wait()

	sort.SliceStable(out, func(i, j int) bool { return out[i].SampleID < out[j].SampleID })
	return out, nil
}

// Summary 是一批检测的汇总。
type Summary struct {
	Submitted int `json:"submitted"`
	// Results 是产出的结果条数，正常情况下等于 Submitted。
	Results  int `json:"results"`
	Positive int `json:"positive"`
	Priority int `json:"priority"`
	// Complete 报告结果条数是否与送检样本数一致。
	Complete bool `json:"complete"`
}

// Summarise 汇总一批检测结果。
func Summarise(samples []model.Sample, results []Result) Summary {
	s := Summary{Submitted: len(samples), Results: len(results)}
	for _, r := range results {
		if r.Positive {
			s.Positive++
		}
		if r.Priority {
			s.Priority++
		}
	}
	s.Complete = s.Results == s.Submitted
	return s
}

// Verify 校验结果条数与送检样本数一致。
func Verify(samples []model.Sample, results []Result) error {
	if len(results) != len(samples) {
		return fmt.Errorf("%w: 送检 %d 份, 结果 %d 条",
			model.ErrLabResultMissing, len(samples), len(results))
	}
	seen := make(map[string]struct{}, len(results))
	for _, r := range results {
		seen[r.SampleID] = struct{}{}
	}
	for _, s := range samples {
		if _, ok := seen[s.ID]; !ok {
			return fmt.Errorf("%w: 样本 %s 缺少结果", model.ErrLabResultMissing, s.ID)
		}
	}
	return nil
}

// Describe 返回检测结果的单行描述。
func Describe(r Result) string {
	verdict := "阴性"
	if r.Positive {
		verdict = "阳性"
	}
	tag := ""
	if r.Priority {
		tag = " [加急]"
	}
	return fmt.Sprintf("%s %s 耗时%dms%s", r.SampleID, verdict, r.ElapsedMS, tag)
}
