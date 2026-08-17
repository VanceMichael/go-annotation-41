package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"portbio/internal/httpapi"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	s, err := httpapi.New()
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}
	return s.Handler()
}

func get(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	payload := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("响应不是合法 JSON: %v\n%s", err, rec.Body.String())
		}
	}
	return rec.Code, payload
}

func TestHealthz(t *testing.T) {
	code, payload := get(t, newServer(t), "/healthz")
	if code != http.StatusOK || payload["ok"] != true {
		t.Fatalf("健康检查异常: %d %v", code, payload)
	}
}

func TestSpeciesEndpoints(t *testing.T) {
	h := newServer(t)
	if code, payload := get(t, h, "/api/species"); code != http.StatusOK || payload["species"] == nil {
		t.Fatalf("名录列表异常: %d %v", code, payload)
	}
	if code, payload := get(t, h, "/api/species/SP-001"); code != http.StatusOK || payload["species"] == nil {
		t.Fatalf("名录详情异常: %d %v", code, payload)
	}
	if code, _ := get(t, h, "/api/species/SP-NOPE"); code != http.StatusNotFound {
		t.Fatalf("不存在的物种应返回 404, 实际 %d", code)
	}
}

func TestQueuePaging(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/queue?page=1&size=4")
	if code != http.StatusOK {
		t.Fatalf("分页查询应返回 200, 实际 %d", code)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("第一页应有 4 条: %v", payload["items"])
	}
}

func TestQueueBadParams(t *testing.T) {
	h := newServer(t)
	if code, _ := get(t, h, "/api/queue?size=0"); code != http.StatusBadRequest {
		t.Fatalf("size 为 0 应返回 400, 实际 %d", code)
	}
	if code, _ := get(t, h, "/api/queue?page=x"); code != http.StatusBadRequest {
		t.Fatalf("page 非法应返回 400, 实际 %d", code)
	}
}

func TestQueueCoverageComplete(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/queue/coverage")
	if code != http.StatusOK {
		t.Fatalf("覆盖核对应返回 200, 实际 %d %v", code, payload)
	}
	cov, ok := payload["coverage"].(map[string]any)
	if !ok || cov["complete"] != true {
		t.Fatalf("逐页遍历应完整: %v", payload)
	}
}

func TestReportEndpoints(t *testing.T) {
	h := newServer(t)
	if code, payload := get(t, h, "/api/report/catalog"); code != http.StatusOK || payload["summary"] == nil {
		t.Fatalf("名录报表异常: %d %v", code, payload)
	}
	if code, payload := get(t, h, "/api/report/queue"); code != http.StatusOK || payload["summary"] == nil {
		t.Fatalf("队列报表异常: %d %v", code, payload)
	}
}

func TestNotifyDisabledIsConflict(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/notify?channel=disabled")
	if code != http.StatusConflict {
		t.Fatalf("通道停用应返回 409, 实际 %d %v", code, payload)
	}
}

func TestNotifyThrottledSucceeds(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/notify?channel=throttled")
	if code != http.StatusOK || payload["outcome"] == nil {
		t.Fatalf("限流后应重试成功: %d %v", code, payload)
	}
}

func TestNotifyUnknownChannel(t *testing.T) {
	code, _ := get(t, newServer(t), "/api/notify?channel=zz")
	if code != http.StatusBadRequest {
		t.Fatalf("未知通道应返回 400, 实际 %d", code)
	}
}
