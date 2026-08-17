// Package cli 实现 bioctl 命令行。
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"portbio/internal/dispose"
	"portbio/internal/httpapi"
	"portbio/internal/lab"
	"portbio/internal/model"
	"portbio/internal/notify"
	"portbio/internal/queue"
	"portbio/internal/report"
	"portbio/internal/seed"
	"portbio/internal/species"
	"portbio/internal/sweep"
)

// 退出码约定，与 README 保持一致。
const (
	// ExitOK 成功。
	ExitOK = 0
	// ExitUsage 用法错误或未归类的内部错误。
	ExitUsage = 1
	// ExitInvalidArg 参数非法。
	ExitInvalidArg = 2
	// ExitConflict 业务冲突。
	ExitConflict = 3
	// ExitAborted 调用被取消、超时或通道限流。
	ExitAborted = 4
	// ExitNotFound 资源不存在。
	ExitNotFound = 5
	// ExitDataIssue 数据一致性问题。
	ExitDataIssue = 6
)

// classify 把错误映射为退出码。
func classify(err error) int {
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, model.ErrSpeciesNotFound),
		errors.Is(err, model.ErrInterceptionNotFound):
		return ExitNotFound
	case errors.Is(err, model.ErrMeasureNotPermitted),
		errors.Is(err, model.ErrNotifyChannelDisabled),
		errors.Is(err, model.ErrNotifyRejected):
		return ExitConflict
	case errors.Is(err, model.ErrSweepAborted),
		errors.Is(err, model.ErrNotifyThrottled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled):
		return ExitAborted
	case errors.Is(err, model.ErrHandlerMissing),
		errors.Is(err, model.ErrQueuePagination),
		errors.Is(err, model.ErrLabBatchFailed),
		errors.Is(err, model.ErrLabResultMissing):
		return ExitDataIssue
	case errors.Is(err, model.ErrUnknownRisk),
		errors.Is(err, model.ErrUnknownMeasure),
		errors.Is(err, model.ErrUnknownChannel),
		errors.Is(err, model.ErrInvalidSpecies),
		errors.Is(err, model.ErrInvalidInterception),
		errors.Is(err, model.ErrInvalidSample):
		return ExitInvalidArg
	default:
		return ExitUsage
	}
}

type app struct {
	out io.Writer
	err io.Writer
}

// Run 执行一次命令行调用并返回退出码。
func Run(args []string, out, errOut io.Writer) int {
	a := &app{out: out, err: errOut}
	if len(args) < 2 {
		a.usage()
		return ExitUsage
	}
	code, err := a.route(args[1:])
	if err != nil {
		fmt.Fprintf(a.err, "错误: %v\n", err)
	}
	return code
}

func (a *app) usage() {
	fmt.Fprint(a.err, `bioctl —— 口岸外来物种查验与生物安全处置

用法:
  bioctl species list
  bioctl species show --code SP-001
  bioctl queue list [--page 1 --size 4]
  bioctl queue verify [--size 4]
  bioctl dispose run --id IC-001
  bioctl dispose batch
  bioctl sweep run --timeout 100ms --per-item 20ms
  bioctl lab run --samples 12
  bioctl notify report --channel disabled
  bioctl notify report --channel throttled --limit 2
  bioctl report catalog|queue|disposal
  bioctl serve --addr 127.0.0.1:8080
  bioctl selfcheck

退出码: 0 成功 / 1 用法或未归类 / 2 参数非法 / 3 业务冲突 /
        4 调用被取消、超时或通道限流 / 5 资源不存在 / 6 数据一致性问题
`)
}

func (a *app) route(args []string) (int, error) {
	switch args[0] {
	case "species":
		return a.runSpecies(args[1:])
	case "queue":
		return a.runQueue(args[1:])
	case "dispose":
		return a.runDispose(args[1:])
	case "sweep":
		return a.runSweep(args[1:])
	case "lab":
		return a.runLab(args[1:])
	case "notify":
		return a.runNotify(args[1:])
	case "report":
		return a.runReport(args[1:])
	case "serve":
		return a.runServe(args[1:])
	case "selfcheck":
		return a.runSelfcheck()
	case "help", "-h", "--help":
		a.usage()
		return ExitOK, nil
	default:
		a.usage()
		return ExitUsage, fmt.Errorf("未知子命令 %q", args[0])
	}
}

func (a *app) emit(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// buildCatalog 构造物种名录。
func buildCatalog() (*species.Catalog, error) {
	c := species.NewCatalog()
	if err := c.AddAll(seed.SpeciesList()); err != nil {
		return nil, err
	}
	return c, nil
}

// buildQueue 构造查验队列。
func buildQueue() (*queue.Queue, error) {
	q := queue.New()
	if err := q.AddAll(seed.Interceptions()); err != nil {
		return nil, err
	}
	return q, nil
}

// buildDispose 构造处置服务，并注册本口岸支持的处置器。
//
// 「送实验室隔离检测」需上级授权，本口岸不注册该措施的处置器。
func buildDispose() (*dispose.Service, error) {
	svc := dispose.NewService(seed.PortCode())
	mk := func(note string) dispose.Handler {
		return func(it model.Interception) (dispose.Disposal, error) {
			return dispose.Disposal{
				InterceptionID: it.ID,
				Measure:        it.Measure,
				Handled:        it.Quantity,
				Note:           note,
			}, nil
		}
	}
	for m, note := range map[model.Measure]string{
		model.MeasureRelease:   "查验合格放行",
		model.MeasureDisinfect: "消杀处理后放行",
		model.MeasureReturn:    "已出具退运通知",
		model.MeasureDestroy:   "已监督销毁",
	} {
		if err := svc.Register(m, mk(note)); err != nil {
			return nil, err
		}
	}
	return svc, nil
}

func (a *app) runSpecies(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("species 需要子命令 list 或 show")
	}
	c, err := buildCatalog()
	if err != nil {
		return classify(err), err
	}
	switch args[0] {
	case "list":
		items := c.List()
		lines := make([]string, 0, len(items))
		for _, s := range items {
			lines = append(lines, species.Describe(s))
		}
		if eerr := a.emit(map[string]any{
			"species": items, "describe": lines, "count": len(items),
			"quarantine": c.QuarantineCodes(),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "show":
		fs := flag.NewFlagSet("species show", flag.ContinueOnError)
		fs.SetOutput(a.err)
		code := fs.String("code", "SP-001", "名录编号")
		if perr := fs.Parse(args[1:]); perr != nil {
			return ExitInvalidArg, perr
		}
		s, gerr := c.Get(*code)
		if gerr != nil {
			return classify(gerr), gerr
		}
		if eerr := a.emit(map[string]any{
			"species": s, "describe": species.Describe(s),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	default:
		return ExitUsage, fmt.Errorf("未知 species 子命令 %q", args[0])
	}
}

func (a *app) runQueue(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("queue 需要子命令 list 或 verify")
	}
	q, err := buildQueue()
	if err != nil {
		return classify(err), err
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("queue list", flag.ContinueOnError)
		fs.SetOutput(a.err)
		page := fs.Int("page", 1, "页码，从 1 开始")
		size := fs.Int("size", seed.PageSize(), "页大小")
		if perr := fs.Parse(args[1:]); perr != nil {
			return ExitInvalidArg, perr
		}
		items := q.Page(*page, *size)
		lines := make([]string, 0, len(items))
		for _, it := range items {
			lines = append(lines, queue.Describe(it))
		}
		if eerr := a.emit(map[string]any{
			"page": *page, "size": *size, "pages": q.Pages(*size),
			"total": q.Len(), "items": items, "describe": lines, "count": len(items),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "verify":
		return a.runQueueVerify(q, args[1:])
	default:
		return ExitUsage, fmt.Errorf("未知 queue 子命令 %q", args[0])
	}
}

func (a *app) runQueueVerify(q *queue.Queue, args []string) (int, error) {
	fs := flag.NewFlagSet("queue verify", flag.ContinueOnError)
	fs.SetOutput(a.err)
	size := fs.Int("size", seed.PageSize(), "页大小")
	rounds := fs.Int("rounds", 5, "重复核对轮数")
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}
	if *size <= 0 || *rounds <= 0 {
		err := fmt.Errorf("%w: size 与 rounds 必须为正", model.ErrInvalidInterception)
		return classify(err), err
	}

	covs := make([]queue.Coverage, 0, *rounds)
	bad := 0
	for r := 0; r < *rounds; r++ {
		cov := q.CheckCoverage(*size)
		if !cov.Complete {
			bad++
		}
		covs = append(covs, cov)
	}

	// 排序口径必须稳定：重复取第一页应当得到完全相同的顺序。
	firstPage := q.Page(1, *size)
	stable := true
	for r := 0; r < *rounds; r++ {
		again := q.Page(1, *size)
		if len(again) != len(firstPage) {
			stable = false
			break
		}
		for i := range again {
			if again[i].ID != firstPage[i].ID {
				stable = false
				break
			}
		}
		if !stable {
			break
		}
	}

	ok := bad == 0 && stable
	if eerr := a.emit(map[string]any{
		"total":               q.Len(),
		"size":                *size,
		"pages":               q.Pages(*size),
		"rounds":              *rounds,
		"incomplete_rounds":   bad,
		"first_page_stable":   stable,
		"coverage":            covs[0],
		"coverage_all_rounds": covs,
		"ok":                  ok,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if !ok {
		if !stable {
			err := fmt.Errorf("%w: 重复取第一页得到的顺序不一致", model.ErrQueuePagination)
			return classify(err), err
		}
		return classify(q.Verify(*size)), q.Verify(*size)
	}
	return ExitOK, nil
}

func (a *app) runDispose(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("dispose 需要子命令 run 或 batch")
	}
	svc, err := buildDispose()
	if err != nil {
		return classify(err), err
	}
	switch args[0] {
	case "run":
		fs := flag.NewFlagSet("dispose run", flag.ContinueOnError)
		fs.SetOutput(a.err)
		id := fs.String("id", "IC-001", "截获编号")
		if perr := fs.Parse(args[1:]); perr != nil {
			return ExitInvalidArg, perr
		}
		q, qerr := buildQueue()
		if qerr != nil {
			return classify(qerr), qerr
		}
		it, gerr := q.Get(*id)
		if gerr != nil {
			return classify(gerr), gerr
		}
		d, derr := svc.Dispose(it)
		if derr != nil {
			if eerr := a.emit(map[string]any{
				"interception_id": it.ID, "measure": string(it.Measure),
				"ok": false, "exit_code": classify(derr), "message": derr.Error(),
				"registered": svc.Registered(),
			}); eerr != nil {
				return ExitUsage, eerr
			}
			return classify(derr), derr
		}
		if eerr := a.emit(map[string]any{
			"interception_id": it.ID, "ok": true,
			"disposal": d, "describe": dispose.Describe(d),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "batch":
		items := seed.Interceptions()
		done, skipped, derr := svc.DisposeAll(items)
		if derr != nil {
			return classify(derr), derr
		}
		sort.Strings(skipped)
		if eerr := a.emit(map[string]any{
			"submitted":        len(items),
			"handled":          len(done),
			"skipped":          len(skipped),
			"skipped_ids":      skipped,
			"registered":       svc.Registered(),
			"unregistered":     string(seed.UnregisteredMeasure()),
			"handled_quantity": svc.HandledQuantity(),
			"ok":               len(done)+len(skipped) == len(items),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		if len(done)+len(skipped) != len(items) {
			err := fmt.Errorf("%w: 提交 %d 条, 处置 %d 条, 跳过 %d 条",
				model.ErrHandlerMissing, len(items), len(done), len(skipped))
			return classify(err), err
		}
		return ExitOK, nil
	default:
		return ExitUsage, fmt.Errorf("未知 dispose 子命令 %q", args[0])
	}
}

func (a *app) runSweep(args []string) (int, error) {
	if len(args) == 0 || args[0] != "run" {
		return ExitUsage, fmt.Errorf("sweep 需要子命令 run")
	}
	fs := flag.NewFlagSet("sweep run", flag.ContinueOnError)
	fs.SetOutput(a.err)
	timeout := fs.Duration("timeout", 100*time.Millisecond, "批量复核的整体超时")
	perItem := fs.Duration("per-item", 20*time.Millisecond, "单条记录的复核耗时")
	if perr := fs.Parse(args[1:]); perr != nil {
		return ExitInvalidArg, perr
	}

	items := seed.Interceptions()
	sw := sweep.New(*perItem)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	rep := sw.SweepBatch(ctx, items)
	budget := int64(*timeout / time.Millisecond)
	// 中止应当发生在超时附近，而不是把剩余记录全部跑完。
	withinBudget := rep.ElapsedMS <= budget+int64(*perItem/time.Millisecond)+50
	ok := rep.Aborted && rep.Checked < rep.Submitted && withinBudget

	if eerr := a.emit(map[string]any{
		"submitted":     rep.Submitted,
		"checked":       rep.Checked,
		"flagged":       rep.Flagged,
		"aborted":       rep.Aborted,
		"elapsed_ms":    rep.ElapsedMS,
		"timeout_ms":    budget,
		"per_item_ms":   int64(*perItem / time.Millisecond),
		"within_budget": withinBudget,
		"message":       rep.Message,
		"ok":            ok,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if !ok {
		err := fmt.Errorf("%w: 超时 %dms, 实际耗时 %dms, 复核 %d/%d 条, 是否中止=%v",
			model.ErrSweepAborted, budget, rep.ElapsedMS, rep.Checked, rep.Submitted, rep.Aborted)
		return ExitDataIssue, err
	}
	err := fmt.Errorf("%w: %s", model.ErrSweepAborted, rep.Message)
	return classify(err), err
}

func (a *app) runLab(args []string) (int, error) {
	if len(args) == 0 || args[0] != "run" {
		return ExitUsage, fmt.Errorf("lab 需要子命令 run")
	}
	fs := flag.NewFlagSet("lab run", flag.ContinueOnError)
	fs.SetOutput(a.err)
	n := fs.Int("samples", 12, "送检样本数")
	workers := fs.Int("workers", 4, "并发工位数")
	if perr := fs.Parse(args[1:]); perr != nil {
		return ExitInvalidArg, perr
	}
	if *n <= 0 {
		err := fmt.Errorf("%w: samples 必须为正", model.ErrInvalidSample)
		return classify(err), err
	}

	samples := seed.Samples(*n)
	results, rerr := lab.New(*workers).RunBatch(samples)
	if rerr != nil {
		return classify(rerr), rerr
	}
	sum := lab.Summarise(samples, results)
	verr := lab.Verify(samples, results)

	lines := make([]string, 0, len(results))
	for _, r := range results {
		lines = append(lines, lab.Describe(r))
	}
	if eerr := a.emit(map[string]any{
		"submitted":       sum.Submitted,
		"results":         sum.Results,
		"positive":        sum.Positive,
		"priority":        sum.Priority,
		"priority_seeded": seed.PrioritySampleCount(*n),
		"complete":        sum.Complete,
		"describe":        lines,
		"ok":              verr == nil,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if verr != nil {
		return classify(verr), verr
	}
	return ExitOK, nil
}

func (a *app) runNotify(args []string) (int, error) {
	if len(args) == 0 || args[0] != "report" {
		return ExitUsage, fmt.Errorf("notify 需要子命令 report")
	}
	fs := flag.NewFlagSet("notify report", flag.ContinueOnError)
	fs.SetOutput(a.err)
	kind := fs.String("channel", "disabled", "演练通道: throttled | disabled")
	limit := fs.Int("limit", 2, "throttled 通道的前置限流次数")
	id := fs.String("id", "IC-001", "截获编号")
	if perr := fs.Parse(args[1:]); perr != nil {
		return ExitInvalidArg, perr
	}

	var ch notify.Channel
	var calls func() int
	switch *kind {
	case "throttled":
		c := notify.NewThrottledChannel(*limit)
		ch, calls = c, c.Calls
	case "disabled":
		c := notify.NewDisabledChannel()
		ch, calls = c, c.Calls
	default:
		err := fmt.Errorf("%w: 未知演练通道 %q", model.ErrUnknownChannel, *kind)
		return classify(err), err
	}

	start := time.Now()
	out := notify.NewReporter(ch).Report(*id)
	elapsed := time.Since(start)
	cerr := notify.Classify(out)

	payload := map[string]any{
		"channel":       *kind,
		"limit":         *limit,
		"ok":            out.OK,
		"calls":         out.Calls,
		"channel_calls": calls(),
		"retries":       out.Retries,
		"attempts":      out.Attempts,
		"elapsed_ms":    elapsed.Milliseconds(),
		"describe":      notify.Describe(out),
	}
	if cerr != nil {
		payload["exit_code"] = classify(cerr)
		payload["message"] = cerr.Error()
	}
	if eerr := a.emit(payload); eerr != nil {
		return ExitUsage, eerr
	}
	if cerr != nil {
		return classify(cerr), cerr
	}
	return ExitOK, nil
}

func (a *app) runReport(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("report 需要子命令 catalog、queue 或 disposal")
	}
	switch args[0] {
	case "catalog":
		c, err := buildCatalog()
		if err != nil {
			return classify(err), err
		}
		sum := report.Catalog(c)
		if eerr := a.emit(map[string]any{
			"summary": sum, "describe": report.Describe(sum),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "queue":
		q, err := buildQueue()
		if err != nil {
			return classify(err), err
		}
		sum := report.Queue(q, seed.PageSize())
		if eerr := a.emit(map[string]any{
			"summary": sum, "ok": sum.Coverage.Complete,
		}); eerr != nil {
			return ExitUsage, eerr
		}
		if !sum.Coverage.Complete {
			err := q.Verify(seed.PageSize())
			return classify(err), err
		}
		return ExitOK, nil
	case "disposal":
		svc, err := buildDispose()
		if err != nil {
			return classify(err), err
		}
		rows, sum, derr := report.Disposal(svc, seed.Interceptions())
		if derr != nil {
			return classify(derr), derr
		}
		if eerr := a.emit(map[string]any{
			"rows": rows, "summary": sum, "ok": true,
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	default:
		return ExitUsage, fmt.Errorf("未知 report 子命令 %q", args[0])
	}
}

func (a *app) runServe(args []string) (int, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(a.err)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址")
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}
	srv, err := httpapi.New()
	if err != nil {
		return classify(err), err
	}
	fmt.Fprintf(a.err, "bioctl 正在监听 %s（接口无鉴权，仅供内网或本地演练）\n", *addr)
	if lerr := http.ListenAndServe(*addr, srv.Handler()); lerr != nil {
		return ExitUsage, lerr
	}
	return ExitOK, nil
}

func (a *app) runSelfcheck() (int, error) {
	type item struct {
		Name string `json:"name"`
		OK   bool   `json:"ok"`
		Note string `json:"note,omitempty"`
	}
	checks := make([]item, 0, 10)
	add := func(name string, ok bool, note string) {
		checks = append(checks, item{Name: name, OK: ok, Note: note})
	}
	note := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	c, cerr := buildCatalog()
	add("物种名录装载", cerr == nil, note(cerr))

	q, qerr := buildQueue()
	add("查验队列装载", qerr == nil, note(qerr))
	if qerr == nil {
		add("分页逐页覆盖完整", q.Verify(seed.PageSize()) == nil, note(q.Verify(seed.PageSize())))
		first := q.Page(1, seed.PageSize())
		again := q.Page(1, seed.PageSize())
		same := len(first) == len(again)
		for i := range first {
			if same && first[i].ID != again[i].ID {
				same = false
			}
		}
		add("分页顺序稳定", same, "")
	}

	svc, derr := buildDispose()
	add("处置服务装载", derr == nil, note(derr))
	if derr == nil {
		it := model.Interception{
			ID: "SELFCHECK", PortCode: seed.PortCode(), SpeciesCode: "SP-001",
			Channel: model.ChannelMail, Risk: model.RiskQuarantine,
			Measure: seed.UnregisteredMeasure(), Quantity: 1, AcceptedAt: seed.Now(),
		}
		_, herr := svc.Dispose(it)
		add("未注册处置器返回哨兵错误", errors.Is(herr, model.ErrHandlerMissing), note(herr))

		done, skipped, berr := svc.DisposeAll(seed.Interceptions())
		add("批量处置不中断",
			berr == nil && len(done)+len(skipped) == len(seed.Interceptions()), note(berr))
	}

	// 批量复核应在超时附近中止。
	sw := sweep.New(20 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	rep := sw.SweepBatch(ctx, seed.Interceptions())
	cancel()
	add("批量复核及时中止",
		rep.Aborted && rep.Checked < rep.Submitted && rep.ElapsedMS < 400,
		fmt.Sprintf("复核 %d/%d 条, 耗时 %dms", rep.Checked, rep.Submitted, rep.ElapsedMS))

	// 实验室并发检测结果条数与送检数一致。
	samples := seed.Samples(12)
	results, lerr := lab.New(4).RunBatch(samples)
	add("实验室批次可执行", lerr == nil, note(lerr))
	add("实验室结果条数一致", lab.Verify(samples, results) == nil,
		note(lab.Verify(samples, results)))

	// 确定性通报错误不应重试。
	dc := notify.NewDisabledChannel()
	dout := notify.NewReporter(dc).Report("IC-001")
	add("通道停用不重试", dout.Calls == 1 && dout.Retries == 0,
		fmt.Sprintf("调用 %d 次, 重试 %d 次", dout.Calls, dout.Retries))

	// 限流通报应重试并最终成功。
	tc := notify.NewThrottledChannel(2)
	tout := notify.NewReporter(tc).Report("IC-001")
	add("限流通报重试后成功", tout.OK && tout.Retries == 2,
		fmt.Sprintf("成功=%v, 重试 %d 次", tout.OK, tout.Retries))

	if cerr == nil {
		add("检疫名单条数", len(c.QuarantineCodes()) == 2, "")
	}

	failed := 0
	for _, x := range checks {
		if !x.OK {
			failed++
		}
	}
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].Name < checks[j].Name })
	if eerr := a.emit(map[string]any{
		"checks": checks, "passed": len(checks) - failed, "failed": failed, "ok": failed == 0,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if failed > 0 {
		return ExitDataIssue, fmt.Errorf("自检未通过: %d 项失败", failed)
	}
	return ExitOK, nil
}
