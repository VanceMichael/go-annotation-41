# portbio —— 口岸外来物种查验与生物安全处置平台

`portbio` 是一个纯 Go 实现的后端与命令行工具，覆盖物种名录与风险分级、
进境查验截获登记、查验队列排序与分页、批量复核、实验室并发检测与疫情通报上报。

项目不依赖任何第三方模块，只使用 Go 标准库。

## 业务背景

### 风险分级与处置措施

风险等级由低到高为 `low` < `medium` < `high` < `quarantine`。
处置措施与风险等级的匹配约束：**检疫性有害生物不得放行；高风险及以上不得仅消杀后放行**。

各口岸能执行的处置措施并不相同。**未在本口岸注册处置器的措施必须返回
`model.ErrHandlerMissing`**，交由上级授权后再处理，不得因为缺少处置器而崩溃。
批量处置时这类记录被跳过并计入 `skipped`，不中断其余记录的处置。

### 查验队列排序与分页

查验队列的唯一排序口径是「**风险等级降序、受理时间升序、编号升序**」：
风险高的先查，同风险先到先查。同一份队列多次排序必须得到完全一致的顺序。

分页建立在这个稳定顺序之上：**逐页取完必须恰好覆盖全部记录，既不重复也不遗漏**；
重复取同一页必须得到相同的顺序。页码越界时返回空页。

### 批量复核与取消传播

批量复核逐条处理截获记录。**每处理一条记录之前都必须检查调用方 context 是否已结束**：
已结束时立即停止，返回已完成条数与可沿错误链判定为 `model.ErrSweepAborted` 的错误，
不得把剩余记录继续跑完。中止应当发生在超时附近，而不是等整批跑完。

### 实验室并发检测

加急样本走快速通道，普通样本走常规通道。无论走哪条通道，
**每份样本都必须产出且只产出一条结果：返回的结果条数必须等于送检样本数**。

### 通报上报与重试

通报失败分两类：

- **可重试**：`model.ErrNotifyThrottled`（通道调用频次超限），按退避重试，最多 5 次。
- **确定性**：`model.ErrNotifyChannelDisabled`（通道已停用）、`model.ErrNotifyRejected`
  （内容被拒收），**必须立即返回，不做任何重试**。

判定必须沿错误链用 `errors.Is` 完成，不得依赖错误文本。
最终归类也据此区分：确定性失败归为通道停用/拒收，重试耗尽才归为限流。

## 目录结构

```
cmd/bioctl              命令行入口
internal/model          领域模型与哨兵错误
internal/species        物种名录与风险分级
internal/queue          查验队列排序与分页
internal/dispose        处置措施派发与执行
internal/sweep          批量复核与取消传播
internal/lab            实验室样本并发检测
internal/notify         通报上报与退避重试
internal/report         名录、队列与处置报表
internal/httpapi        HTTP 接口
internal/seed           内置样例数据
internal/cli            bioctl 命令实现
```

## 构建与测试

```bash
export GOTOOLCHAIN=local

go build ./...
go test ./...
go test -race ./...

make build           # 产出 bin/bioctl
make selfcheck       # 构建并运行内置自检
```

## 命令行用法

```bash
bioctl species list
bioctl species show --code SP-001

bioctl queue list --page 1 --size 4
bioctl queue verify --size 4          # 核对逐页遍历的覆盖完整性与顺序稳定性

bioctl dispose run --id IC-001
bioctl dispose run --id IC-009        # 本口岸未注册该措施的处置器
bioctl dispose batch

bioctl sweep run --timeout 100ms --per-item 20ms
bioctl lab run --samples 12

bioctl notify report --channel disabled              # 确定性错误，不应重试
bioctl notify report --channel throttled --limit 2   # 可重试错误，退避后成功

bioctl report catalog|queue|disposal
bioctl serve --addr 127.0.0.1:8080
bioctl selfcheck
```

`queue verify` 会重复多轮逐页遍历，核对 `coverage.complete` 与 `first_page_stable`。
`sweep run` 给出的超时刻意小于「记录条数 × 单条耗时」，用于校验取消能及时生效。

### 退出码约定

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 用法错误或未归类的内部错误 |
| 2 | 参数非法 |
| 3 | 业务冲突（措施与风险等级不匹配、通报通道停用、通报被拒收） |
| 4 | 调用被取消、超时或通道限流 |
| 5 | 资源不存在 |
| 6 | 数据一致性问题（未注册处置器、分页不完整、实验室结果缺失） |

## HTTP 接口

```
GET /healthz
GET /api/species
GET /api/species/{code}
GET /api/queue[?page=1&size=4]
GET /api/queue/coverage
GET /api/report/catalog
GET /api/report/queue
GET /api/notify[?channel=disabled|throttled]
```

错误响应统一为：

```json
{ "error": { "code": "handler_missing", "message": "..." } }
```

状态码约定：`404` 资源不存在，`409` 业务冲突，`422` 数据一致性问题，
`429` 通道限流，`400` 参数非法，`503` 调用被取消或超时，`500` 未归类的内部错误。

> 说明：HTTP 接口默认不带鉴权，仅面向内网或本地演练环境。若需暴露到公网，
> 必须在前置网关补充身份认证与访问控制。

## 容器运行

```bash
docker build -t portbio:local .
docker run --rm portbio:local selfcheck
docker run --rm -p 8080:8080 portbio:local serve --addr 0.0.0.0:8080
```

镜像基于 `golang:1.22` 构建、`distroless/static` 运行，同时支持
`linux/amd64` 与 `linux/arm64`：

```bash
docker build --platform linux/amd64 -t portbio:amd64 .
docker build --platform linux/arm64 -t portbio:arm64 .
```

## 数据来源

内置样例数据（`internal/seed`）包含 7 条物种名录（其中 2 条列入检疫性有害生物名单）、
15 条截获记录（其中 2 条的处置措施在本口岸未注册处置器）与 12 份送检样本（含加急样本）。
仅用于本地演练，不代表真实查验数据。
