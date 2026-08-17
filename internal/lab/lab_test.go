package lab_test

import (
	"errors"
	"testing"

	"portbio/internal/lab"
	"portbio/internal/model"
	"portbio/internal/seed"
)

func TestRunBatchReturnsResultForEverySample(t *testing.T) {
	samples := seed.Samples(12)
	results, err := lab.New(4).RunBatch(samples)
	if err != nil {
		t.Fatalf("批次检测不应失败: %v", err)
	}
	if len(results) != len(samples) {
		t.Fatalf("结果条数应为 %d, 实际 %d", len(samples), len(results))
	}
	if err := lab.Verify(samples, results); err != nil {
		t.Fatalf("结果应覆盖全部送检样本: %v", err)
	}
}

func TestRunBatchWithPrioritySamples(t *testing.T) {
	samples := seed.Samples(12)
	if seed.PrioritySampleCount(12) == 0 {
		t.Fatal("样例样本应包含加急样本")
	}
	results, err := lab.New(4).RunBatch(samples)
	if err != nil {
		t.Fatalf("批次检测不应失败: %v", err)
	}
	sum := lab.Summarise(samples, results)
	if sum.Priority != seed.PrioritySampleCount(12) {
		t.Fatalf("加急样本结果应为 %d 条, 实际 %d 条",
			seed.PrioritySampleCount(12), sum.Priority)
	}
	if !sum.Complete {
		t.Fatalf("结果应完整: %+v", sum)
	}
}

func TestRunBatchAllPrioritySamples(t *testing.T) {
	samples := make([]model.Sample, 0, 8)
	for i := 1; i <= 8; i++ {
		samples = append(samples, model.Sample{
			ID: seed.Samples(8)[i-1].ID, InterceptionID: "IC-001",
			Priority: true, AssayMS: 1,
		})
	}
	results, err := lab.New(4).RunBatch(samples)
	if err != nil {
		t.Fatalf("全加急批次不应失败: %v", err)
	}
	if len(results) != len(samples) {
		t.Fatalf("全加急批次结果应为 %d 条, 实际 %d 条", len(samples), len(results))
	}
}

func TestRunBatchVerifyComplete(t *testing.T) {
	for _, n := range []int{1, 3, 6, 12, 20} {
		samples := seed.Samples(n)
		results, err := lab.New(4).RunBatch(samples)
		if err != nil {
			t.Fatalf("样本数 %d: 检测失败: %v", n, err)
		}
		if err := lab.Verify(samples, results); err != nil {
			t.Fatalf("样本数 %d: %v", n, err)
		}
	}
}

func TestRunBatchRepeatedIsStable(t *testing.T) {
	samples := seed.Samples(12)
	l := lab.New(4)
	first, err := l.RunBatch(samples)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}
	for r := 0; r < 5; r++ {
		again, err := l.RunBatch(samples)
		if err != nil {
			t.Fatalf("第 %d 次检测失败: %v", r+1, err)
		}
		if len(again) != len(first) {
			t.Fatalf("第 %d 次结果条数 %d 与首次 %d 不一致", r+1, len(again), len(first))
		}
		for i := range again {
			if again[i].SampleID != first[i].SampleID {
				t.Fatalf("第 %d 次结果顺序不稳定", r+1)
			}
		}
	}
}

func TestRunBatchRejectsInvalidSample(t *testing.T) {
	_, err := lab.New(4).RunBatch([]model.Sample{{ID: "", InterceptionID: "IC-001"}})
	if !errors.Is(err, model.ErrInvalidSample) {
		t.Fatalf("非法样本应返回 ErrInvalidSample, 得到 %v", err)
	}
}

func TestVerifyDetectsMissingResult(t *testing.T) {
	samples := seed.Samples(6)
	results, err := lab.New(4).RunBatch(samples)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}
	if verr := lab.Verify(samples, results[:len(results)-1]); !errors.Is(verr, model.ErrLabResultMissing) {
		t.Fatalf("缺少结果应返回 ErrLabResultMissing, 得到 %v", verr)
	}
}

func TestSummariseCountsPositives(t *testing.T) {
	samples := seed.Samples(12)
	results, err := lab.New(4).RunBatch(samples)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}
	sum := lab.Summarise(samples, results)
	want := 0
	for _, r := range results {
		if r.Positive {
			want++
		}
	}
	if sum.Positive != want {
		t.Fatalf("阳性数应为 %d, 实际 %d", want, sum.Positive)
	}
}

func TestRunBatchEmptyInput(t *testing.T) {
	results, err := lab.New(4).RunBatch(nil)
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("空输入应返回 0 条结果, 实际 %d", len(results))
	}
}
