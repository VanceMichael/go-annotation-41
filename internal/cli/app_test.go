package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"portbio/internal/cli"
)

func run(t *testing.T, args ...string) (int, map[string]any, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(append([]string{"bioctl"}, args...), &out, &errOut)
	payload := map[string]any{}
	if s := strings.TrimSpace(out.String()); s != "" {
		if err := json.Unmarshal([]byte(s), &payload); err != nil {
			t.Fatalf("输出不是合法 JSON: %v\n%s", err, s)
		}
	}
	return code, payload, errOut.String()
}

func TestCLISelfcheckPasses(t *testing.T) {
	code, payload, stderr := run(t, "selfcheck")
	if code != cli.ExitOK {
		t.Fatalf("自检退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["ok"] != true {
		t.Fatalf("自检应全部通过: %v", payload)
	}
}

func TestCLIDisposeBatchSkipsUnregistered(t *testing.T) {
	code, payload, stderr := run(t, "dispose", "batch")
	if code != cli.ExitOK {
		t.Fatalf("批量处置退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["skipped"] != float64(2) {
		t.Fatalf("应跳过 2 条未注册处置器的记录: %v", payload["skipped"])
	}
	if payload["handled"] != float64(13) {
		t.Fatalf("应完成 13 条处置: %v", payload["handled"])
	}
}

func TestCLIDisposeUnregisteredIsDataIssue(t *testing.T) {
	code, payload, _ := run(t, "dispose", "run", "--id", "IC-009")
	if code != cli.ExitDataIssue {
		t.Fatalf("未注册处置器退出码应为 %d, 实际 %d\n%v", cli.ExitDataIssue, code, payload)
	}
}

func TestCLIDisposeRegisteredSucceeds(t *testing.T) {
	code, payload, stderr := run(t, "dispose", "run", "--id", "IC-001")
	if code != cli.ExitOK {
		t.Fatalf("已注册措施退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["ok"] != true {
		t.Fatalf("处置应成功: %v", payload)
	}
}

func TestCLIQueueVerifyComplete(t *testing.T) {
	code, payload, stderr := run(t, "queue", "verify", "--size", "4")
	if code != cli.ExitOK {
		t.Fatalf("分页核对退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["incomplete_rounds"] != float64(0) {
		t.Fatalf("不应有不完整的核对轮次: %v", payload["incomplete_rounds"])
	}
	if payload["first_page_stable"] != true {
		t.Fatalf("重复取第一页顺序应稳定: %v", payload)
	}
}

func TestCLIQueueListPaged(t *testing.T) {
	code, payload, stderr := run(t, "queue", "list", "--page", "1", "--size", "4")
	if code != cli.ExitOK {
		t.Fatalf("分页列出退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["count"] != float64(4) {
		t.Fatalf("第一页应有 4 条: %v", payload["count"])
	}
}

func TestCLISweepRunAborts(t *testing.T) {
	code, payload, _ := run(t, "sweep", "run", "--timeout", "100ms", "--per-item", "20ms")
	if code != cli.ExitAborted {
		t.Fatalf("超时中止退出码应为 %d, 实际 %d\n%v", cli.ExitAborted, code, payload)
	}
	if payload["aborted"] != true {
		t.Fatalf("应标记为已中止: %v", payload)
	}
	checked, _ := payload["checked"].(float64)
	submitted, _ := payload["submitted"].(float64)
	if checked >= submitted {
		t.Fatalf("中止时复核条数应少于提交条数: %v/%v", checked, submitted)
	}
	if payload["within_budget"] != true {
		t.Fatalf("应在超时预算附近返回: elapsed=%v timeout=%v",
			payload["elapsed_ms"], payload["timeout_ms"])
	}
}

func TestCLILabRunComplete(t *testing.T) {
	code, payload, stderr := run(t, "lab", "run", "--samples", "12")
	if code != cli.ExitOK {
		t.Fatalf("实验室批次退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["results"] != payload["submitted"] {
		t.Fatalf("结果条数应等于送检数: results=%v submitted=%v",
			payload["results"], payload["submitted"])
	}
	if payload["complete"] != true {
		t.Fatalf("结果应完整: %v", payload)
	}
}

func TestCLINotifyDisabledIsConflict(t *testing.T) {
	code, payload, _ := run(t, "notify", "report", "--channel", "disabled")
	if code != cli.ExitConflict {
		t.Fatalf("通道停用退出码应为 %d, 实际 %d\n%v", cli.ExitConflict, code, payload)
	}
	if payload["calls"] != float64(1) {
		t.Fatalf("确定性错误只应调用 1 次: %v", payload["calls"])
	}
	if payload["retries"] != float64(0) {
		t.Fatalf("确定性错误不应重试: %v", payload["retries"])
	}
}

func TestCLINotifyThrottledSucceeds(t *testing.T) {
	code, payload, stderr := run(t, "notify", "report", "--channel", "throttled", "--limit", "2")
	if code != cli.ExitOK {
		t.Fatalf("限流后应重试成功, 退出码 %d\n%s", code, stderr)
	}
	if payload["retries"] != float64(2) {
		t.Fatalf("应重试 2 次: %v", payload["retries"])
	}
}

func TestCLIReportsSucceed(t *testing.T) {
	for _, sub := range []string{"catalog", "queue", "disposal"} {
		code, payload, stderr := run(t, "report", sub)
		if code != cli.ExitOK {
			t.Fatalf("report %s 退出码应为 0, 实际 %d\n%s", sub, code, stderr)
		}
		if len(payload) == 0 {
			t.Fatalf("report %s 应有输出", sub)
		}
	}
}

func TestCLISpeciesNotFound(t *testing.T) {
	code, _, _ := run(t, "species", "show", "--code", "SP-NOPE")
	if code != cli.ExitNotFound {
		t.Fatalf("不存在的物种退出码应为 %d, 实际 %d", cli.ExitNotFound, code)
	}
}

func TestCLIUnknownNotifyChannelIsInvalidArg(t *testing.T) {
	code, _, _ := run(t, "notify", "report", "--channel", "zz")
	if code != cli.ExitInvalidArg {
		t.Fatalf("未知演练通道退出码应为 %d, 实际 %d", cli.ExitInvalidArg, code)
	}
}
