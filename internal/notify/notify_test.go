package notify_test

import (
	"errors"
	"fmt"
	"testing"

	"portbio/internal/model"
	"portbio/internal/notify"
)

func TestRetryableOnlyForThrottled(t *testing.T) {
	throttled := fmt.Errorf("%w: 每分钟仅允许 2 次", model.ErrNotifyThrottled)
	if !notify.Retryable(throttled) {
		t.Fatal("通道频次超限应判定为可重试")
	}
	for name, err := range map[string]error{
		"通道已停用": fmt.Errorf("%w: 请联系上级系统管理员", model.ErrNotifyChannelDisabled),
		"内容被拒收": fmt.Errorf("%w: 字段缺失", model.ErrNotifyRejected),
	} {
		if notify.Retryable(err) {
			t.Fatalf("%s 属于确定性错误, 不应判定为可重试: %v", name, err)
		}
	}
}

func TestReportDoesNotRetryDisabledChannel(t *testing.T) {
	ch := notify.NewDisabledChannel()
	out := notify.NewReporter(ch).Report("IC-001")
	if out.OK {
		t.Fatal("通道停用时通报不应成功")
	}
	if out.Calls != 1 {
		t.Fatalf("确定性错误只应调用通道 1 次, 实际 %d 次", out.Calls)
	}
	if out.Retries != 0 {
		t.Fatalf("确定性错误不应重试, 实际重试 %d 次", out.Retries)
	}
	if ch.Calls() != 1 {
		t.Fatalf("通道实际收到的调用应为 1 次, 实际 %d 次", ch.Calls())
	}
	if len(out.Attempts) != 1 || out.Attempts[0].Retryable {
		t.Fatalf("尝试记录应只有一条且标记为不可重试: %+v", out.Attempts)
	}
}

func TestReportRetriesThrottledUntilSuccess(t *testing.T) {
	ch := notify.NewThrottledChannel(2)
	out := notify.NewReporter(ch).Report("IC-001")
	if !out.OK {
		t.Fatalf("限流两次后应当成功: %s", out.Message)
	}
	if out.Retries != 2 {
		t.Fatalf("应重试 2 次, 实际 %d 次", out.Retries)
	}
	if out.Calls != 3 || ch.Calls() != 3 {
		t.Fatalf("通道应被调用 3 次, 实际 out=%d ch=%d", out.Calls, ch.Calls())
	}
}

func TestReportGivesUpAfterMaxAttempts(t *testing.T) {
	ch := notify.NewThrottledChannel(notify.MaxAttempts + 5)
	out := notify.NewReporter(ch).Report("IC-001")
	if out.OK {
		t.Fatal("一直限流时不应成功")
	}
	if out.Calls != notify.MaxAttempts {
		t.Fatalf("应尝试 %d 次, 实际 %d 次", notify.MaxAttempts, out.Calls)
	}
	if err := notify.Classify(out); !errors.Is(err, model.ErrNotifyThrottled) {
		t.Fatalf("应归类为 ErrNotifyThrottled, 得到 %v", err)
	}
}

func TestClassifyDistinguishesDisabledFromThrottled(t *testing.T) {
	disabled := notify.NewReporter(notify.NewDisabledChannel()).Report("IC-001")
	err := notify.Classify(disabled)
	if !errors.Is(err, model.ErrNotifyChannelDisabled) {
		t.Fatalf("通道停用应归类为 ErrNotifyChannelDisabled, 得到 %v", err)
	}
	if errors.Is(err, model.ErrNotifyThrottled) {
		t.Fatalf("通道停用不应被归类为限流: %v", err)
	}

	throttled := notify.NewReporter(notify.NewThrottledChannel(notify.MaxAttempts + 1)).Report("IC-002")
	terr := notify.Classify(throttled)
	if !errors.Is(terr, model.ErrNotifyThrottled) {
		t.Fatalf("一直限流应归类为 ErrNotifyThrottled, 得到 %v", terr)
	}
}

func TestClassifyNilOnSuccess(t *testing.T) {
	out := notify.NewReporter(notify.NewThrottledChannel(0)).Report("IC-001")
	if !out.OK {
		t.Fatalf("不限流时应成功: %s", out.Message)
	}
	if err := notify.Classify(out); err != nil {
		t.Fatalf("成功时不应返回错误: %v", err)
	}
}

func TestDescribeNotEmpty(t *testing.T) {
	out := notify.NewReporter(notify.NewDisabledChannel()).Report("IC-001")
	if notify.Describe(out) == "" {
		t.Fatal("描述不应为空")
	}
}
