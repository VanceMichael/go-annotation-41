# BENZHI_README

## 项目说明

- 项目：VanceMichael/go-annotation-41
- 项目用途：portbio 是一个纯 Go 实现的后端与命令行工具，覆盖物种名录与风险分级、 进境查验截获登记、查验队列排序与分页、批量复核、实验室并发检测与疫情通报上报。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/bioctl

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-41-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-41-arm64 linux/arm64
docker run -it benzhi-task-41-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-41-arm64:latest
```

## 题目验证命令

1. 预期退出码 0：`go test ./internal/dispose/ ./internal/report/ ./internal/cli/ -run "TestDisposeUnregisteredMeasureReturnsSentinel|TestDisposeUnregisteredAcrossRecords|TestDisposeAllSkipsUnregistered|TestDisposalReportSkipsUnregistered|TestCLIDisposeBatchSkipsUnregistered" -count=1`
2. 预期退出码 0：`go test -buildvcs=false -count=1 ./...`
3. 预期退出码 0：`go build ./... && go vet ./... && gofmt -l .`

## Bug 复现

Bug 现象、触发步骤和完整错误信息见 `BUG_REPRO.md`。
