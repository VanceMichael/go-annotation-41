// Package httpapi 暴露名录、查验队列与处置的查询接口。
//
// 接口默认不带鉴权，仅面向内网或本地演练环境。若需暴露到公网，
// 必须在前置网关补充身份认证与访问控制。
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"portbio/internal/model"
	"portbio/internal/notify"
	"portbio/internal/queue"
	"portbio/internal/report"
	"portbio/internal/seed"
	"portbio/internal/species"
)

// Server 持有名录与查验队列。
type Server struct {
	catalog *species.Catalog
	queue   *queue.Queue
}

// New 构造 HTTP 服务。
func New() (*Server, error) {
	c := species.NewCatalog()
	if err := c.AddAll(seed.SpeciesList()); err != nil {
		return nil, err
	}
	q := queue.New()
	if err := q.AddAll(seed.Interceptions()); err != nil {
		return nil, err
	}
	return &Server{catalog: c, queue: q}, nil
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, model.ErrSpeciesNotFound),
		errors.Is(err, model.ErrInterceptionNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrMeasureNotPermitted),
		errors.Is(err, model.ErrNotifyChannelDisabled),
		errors.Is(err, model.ErrNotifyRejected):
		return http.StatusConflict
	case errors.Is(err, model.ErrNotifyThrottled):
		return http.StatusTooManyRequests
	case errors.Is(err, model.ErrSweepAborted):
		return http.StatusServiceUnavailable
	case errors.Is(err, model.ErrHandlerMissing),
		errors.Is(err, model.ErrQueuePagination),
		errors.Is(err, model.ErrLabResultMissing):
		return http.StatusUnprocessableEntity
	case errors.Is(err, model.ErrUnknownRisk),
		errors.Is(err, model.ErrUnknownMeasure),
		errors.Is(err, model.ErrUnknownChannel),
		errors.Is(err, model.ErrInvalidInterception),
		errors.Is(err, model.ErrInvalidSample):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func codeFor(err error) string {
	switch {
	case errors.Is(err, model.ErrSpeciesNotFound):
		return "species_not_found"
	case errors.Is(err, model.ErrInterceptionNotFound):
		return "interception_not_found"
	case errors.Is(err, model.ErrHandlerMissing):
		return "handler_missing"
	case errors.Is(err, model.ErrQueuePagination):
		return "queue_pagination_incomplete"
	case errors.Is(err, model.ErrNotifyThrottled):
		return "notify_throttled"
	case errors.Is(err, model.ErrNotifyChannelDisabled):
		return "notify_channel_disabled"
	default:
		return "internal_error"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, statusFor(err), map[string]any{
		"error": map[string]string{"code": codeFor(err), "message": err.Error()},
	})
}

// Handler 返回装配好的路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/species", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"species": s.catalog.List(), "quarantine": s.catalog.QuarantineCodes(),
		})
	})

	mux.HandleFunc("/api/species/", func(w http.ResponseWriter, r *http.Request) {
		code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/species/"), "/")
		if code == "" {
			writeErr(w, fmt.Errorf("%w: 缺少名录编号", model.ErrInvalidSpecies))
			return
		}
		sp, err := s.catalog.Get(code)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"species": sp})
	})

	mux.HandleFunc("/api/queue", func(w http.ResponseWriter, r *http.Request) {
		size := seed.PageSize()
		if v := r.URL.Query().Get("size"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeErr(w, fmt.Errorf("%w: size 参数非法", model.ErrInvalidInterception))
				return
			}
			size = n
		}
		page := 1
		if v := r.URL.Query().Get("page"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeErr(w, fmt.Errorf("%w: page 参数非法", model.ErrInvalidInterception))
				return
			}
			page = n
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"page": page, "size": size, "pages": s.queue.Pages(size),
			"total": s.queue.Len(), "items": s.queue.Page(page, size),
		})
	})

	mux.HandleFunc("/api/queue/coverage", func(w http.ResponseWriter, r *http.Request) {
		size := seed.PageSize()
		cov := s.queue.CheckCoverage(size)
		if !cov.Complete {
			writeErr(w, s.queue.Verify(size))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"coverage": cov})
	})

	mux.HandleFunc("/api/report/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"summary": report.Catalog(s.catalog)})
	})

	mux.HandleFunc("/api/report/queue", func(w http.ResponseWriter, r *http.Request) {
		sum := report.Queue(s.queue, seed.PageSize())
		if !sum.Coverage.Complete {
			writeErr(w, s.queue.Verify(seed.PageSize()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"summary": sum})
	})

	mux.HandleFunc("/api/notify", func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("channel")
		if kind == "" {
			kind = "disabled"
		}
		var ch notify.Channel
		switch kind {
		case "throttled":
			ch = notify.NewThrottledChannel(2)
		case "disabled":
			ch = notify.NewDisabledChannel()
		default:
			writeErr(w, fmt.Errorf("%w: 未知演练通道 %q", model.ErrUnknownChannel, kind))
			return
		}
		out := notify.NewReporter(ch).Report("IC-001")
		if err := notify.Classify(out); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"outcome": out})
	})

	return mux
}
