# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

批量处置一跑就崩，整批查验记录卡在那里出不来。

```
$ ./bioctl dispose batch
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]

goroutine 1 [running]:
...
$ echo $?
2
```

样例数据里 15 条截获记录，其中 IC-009、IC-010 的处置措施是「送实验室隔离检测」。这个措施需要上级授权，本口岸没有配置对应的处置器。按 README，这类记录应该返回「处置措施未注册处置器」，批量处置时把它们跳过并计入 `skipped`，其余 13 条照常处置 —— 不该崩。

单条也崩：

```
$ ./bioctl dispose run --id IC-009
panic: runtime error: invalid memory address or nil pointer dereference
$ echo $?
2
```

`report disposal` 因为要遍历全部记录，也崩。

对照现象：

- 已注册措施的记录完全正常：`dispose run --id IC-001`（销毁）退出码 0，能出处置结果。
- `dispose batch` 的输出里本来有个 `registered` 字段，列的就是本口岸已注册的 4 个措施 —— 系统自己知道「送实验室隔离检测」不在其中，可一处置到那两条就崩。
- `queue list` 能正常列出这 15 条记录，包括那两条。

帮我修好，让未注册处置器的措施如实返回哨兵错误、批量处置不中断。已有测试跑一遍不要有回归。

## 含 Bug 版本

- 仓库：VanceMichael/go-annotation-41
- 仓库地址：https://github.com/VanceMichael/go-annotation-41.git
- parent SHA：828265ab30bc6ca972f99594c90ca00ad7b8469f

## 复现步骤

```bash
git clone -- https://github.com/VanceMichael/go-annotation-41.git bug-repro
cd bug-repro
git checkout --detach 828265ab30bc6ca972f99594c90ca00ad7b8469f
go test ./internal/dispose/ ./internal/report/ ./internal/cli/ -run "TestDisposeUnregisteredMeasureReturnsSentinel|TestDisposeUnregisteredAcrossRecords|TestDisposeAllSkipsUnregistered|TestDisposalReportSkipsUnregistered|TestCLIDisposeBatchSkipsUnregistered" -count=1
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/dispose/ ./internal/report/ ./internal/cli/ -run "TestDisposeUnregisteredMeasureReturnsSentinel|TestDisposeUnregisteredAcrossRecords|TestDisposeAllSkipsUnregistered|TestDisposalReportSkipsUnregistered|TestCLIDisposeBatchSkipsUnregistered" -count=1
--- FAIL: TestDisposeUnregisteredMeasureReturnsSentinel (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x50770d]

goroutine 5 [running]:
testing.tRunner.func1.2({0x51fb80, 0x63e700})
	/usr/local/go/src/testing/testing.go:1631 +0x24a
testing.tRunner.func1()
	/usr/local/go/src/testing/testing.go:1634 +0x377
panic({0x51fb80?, 0x63e700?})
	/usr/local/go/src/runtime/panic.go:770 +0x132
portbio/internal/dispose.(*Service).Dispose(0xc000014320, {{0x54148f, 0x7}, {0x54147a, 0x7}, {0x5411b0, 0x6}, {0x540e64, 0x4}, {0x542024, ...}, ...})
	/app/internal/dispose/dispose.go:99 +0x10d
portbio/internal/dispose_test.TestDisposeUnregisteredMeasureReturnsSentinel(0xc000070340)
	/app/internal/dispose/dispose_test.go:47 +0x105
testing.tRunner(0xc000070340, 0x54d868)
	/usr/local/go/src/testing/testing.go:1689 +0xfb
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x390
FAIL	portbio/internal/dispose	0.099s
--- FAIL: TestDisposalReportSkipsUnregistered (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x50800d]

goroutine 18 [running]:
testing.tRunner.func1.2({0x5242a0, 0x645700})
	/usr/local/go/src/testing/testing.go:1631 +0x24a
testing.tRunner.func1()
	/usr/local/go/src/testing/testing.go:1634 +0x377
panic({0x5242a0?, 0x645700?})
	/usr/local/go/src/runtime/panic.go:770 +0x132
portbio/internal/dispose.(*Service).Dispose(0xc0001a0730, {{0x546322, 0x6}, {0x5463ba, 0x7}, {0x5462b6, 0x6}, {0x546692, 0x7}, {0x546f6a, ...}, ...})
	/app/internal/dispose/dispose.go:99 +0x10d
portbio/internal/dispose.(*Service).DisposeAll(0xc0001a0730, {0xc000202008, 0xf, 0x8?})
	/app/internal/dispose/dispose.go:116 +0x165
portbio/internal/report.Disposal(0xc0001a0730, {0xc000202008, 0xf, 0xf})
	/app/internal/report/report.go:106 +0x4ad
portbio/internal/report_test.TestDisposalReportSkipsUnregistered(0xc0001be4e0)
	/app/internal/report/report_test.go:90 +0x4c
testing.tRunner(0xc0001be4e0, 0x552810)
	/usr/local/go/src/testing/testing.go:1689 +0xfb
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x390
FAIL	portbio/internal/report	0.119s
--- FAIL: TestCLIDisposeBatchSkipsUnregistered (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x69b42d]

goroutine 19 [running]:
testing.tRunner.func1.2({0x6e4000, 0x995170})
	/usr/local/go/src/testing/testing.go:1631 +0x24a
testing.tRunner.func1()
	/usr/local/go/src/testing/testing.go:1634 +0x377
panic({0x6e4000?, 0x995170?})
	/usr/local/go/src/runtime/panic.go:770 +0x132
portbio/internal/dispose.(*Service).Dispose(0xc00012c5f0, {{0x734272, 0x6}, {0x734698, 0x7}, {0x734242, 0x6}, {0x734aa4, 0x7}, {0x735827, ...}, ...})
	/app/internal/dispose/dispose.go:99 +0x10d
portbio/internal/dispose.(*Service).DisposeAll(0xc00012c5f0, {0xc000101008, 0xf, 0xc00016e000?})
	/app/internal/dispose/dispose.go:116 +0x165
portbio/internal/cli.(*app).runDispose(0xc000179dd8, {0xc00011d280, 0x1, 0x1})
	/app/internal/cli/app.go:393 +0x949
portbio/internal/cli.(*app).route(0xc000042608?, {0xc00011d270, 0x30?, 0x6d0d40?})
	/app/internal/cli/app.go:128 +0x26e
portbio/internal/cli.Run({0xc00011d260?, 0x7e4c0e?, 0xf?}, {0x7bd5e0?, 0xc00011d200?}, {0x7bd5e0?, 0xc00011d230?})
	/app/internal/cli/app.go:91 +0x85
portbio/internal/cli_test.run(0xc000152680, {0xc000042710, 0x2, 0x7e4c0e?})
	/app/internal/cli/app_test.go:15 +0x127
portbio/internal/cli_test.TestCLIDisposeBatchSkipsUnregistered(0xc000152680)
	/app/internal/cli/app_test.go:36 +0x6a
testing.tRunner(0xc000152680, 0x7676a8)
	/usr/local/go/src/testing/testing.go:1689 +0xfb
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x390
FAIL	portbio/internal/cli	0.047s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/dispose/ ./internal/report/ ./internal/cli/ -run "TestDisposeUnregisteredMeasureReturnsSentinel|TestDisposeUnregisteredAcrossRecords|TestDisposeAllSkipsUnregistered|TestDisposalReportSkipsUnregistered|TestCLIDisposeBatchSkipsUnregistered" -count=1
--- FAIL: TestDisposeUnregisteredMeasureReturnsSentinel (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x10f378]

goroutine 6 [running]:
testing.tRunner.func1.2({0x135a80, 0x2537e0})
	/usr/local/go/src/testing/testing.go:1631 +0x1c4
testing.tRunner.func1()
	/usr/local/go/src/testing/testing.go:1634 +0x33c
panic({0x135a80?, 0x2537e0?})
	/usr/local/go/src/runtime/panic.go:770 +0x124
portbio/internal/dispose.(*Service).Dispose(0x4000010410, {{0x157371, 0x7}, {0x15735c, 0x7}, {0x157098, 0x6}, {0x156d58, 0x4}, {0x157f85, ...}, ...})
	/app/internal/dispose/dispose.go:99 +0x148
portbio/internal/dispose_test.TestDisposeUnregisteredMeasureReturnsSentinel(0x400009e4e0)
	/app/internal/dispose/dispose_test.go:47 +0x10c
testing.tRunner(0x400009e4e0, 0x163770)
	/usr/local/go/src/testing/testing.go:1689 +0xec
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x318
FAIL	portbio/internal/dispose	0.009s
--- FAIL: TestDisposalReportSkipsUnregistered (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x10fc58]

goroutine 18 [running]:
testing.tRunner.func1.2({0x1361a0, 0x2537e0})
	/usr/local/go/src/testing/testing.go:1631 +0x1c4
testing.tRunner.func1()
	/usr/local/go/src/testing/testing.go:1634 +0x33c
panic({0x1361a0?, 0x2537e0?})
	/usr/local/go/src/runtime/panic.go:770 +0x124
portbio/internal/dispose.(*Service).Dispose(0x40000b2730, {{0x15820a, 0x6}, {0x15829c, 0x7}, {0x15819e, 0x6}, {0x1584e1, 0x7}, {0x158ecb, ...}, ...})
	/app/internal/dispose/dispose.go:99 +0x148
portbio/internal/dispose.(*Service).DisposeAll(0x40000b2730, {0x4000080808, 0xf, 0x8?})
	/app/internal/dispose/dispose.go:116 +0x10c
portbio/internal/report.Disposal(0x40000b2730, {0x4000080808, 0xf, 0xf})
	/app/internal/report/report.go:106 +0x368
portbio/internal/report_test.TestDisposalReportSkipsUnregistered(0x40000d04e0)
	/app/internal/report/report_test.go:90 +0x48
testing.tRunner(0x40000d04e0, 0x164710)
	/usr/local/go/src/testing/testing.go:1689 +0xec
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x318
FAIL	portbio/internal/report	0.007s
--- FAIL: TestCLIDisposeBatchSkipsUnregistered (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x26e718]

goroutine 6 [running]:
testing.tRunner.func1.2({0x2c2d80, 0x5721d0})
	/usr/local/go/src/testing/testing.go:1631 +0x1c4
testing.tRunner.func1()
	/usr/local/go/src/testing/testing.go:1634 +0x33c
panic({0x2c2d80?, 0x5721d0?})
	/usr/local/go/src/runtime/panic.go:770 +0x124
portbio/internal/dispose.(*Service).Dispose(0x40000126e0, {{0x312ef2, 0x6}, {0x313300, 0x7}, {0x312ec2, 0x6}, {0x313679, 0x7}, {0x3144d2, ...}, ...})
	/app/internal/dispose/dispose.go:99 +0x148
portbio/internal/dispose.(*Service).DisposeAll(0x40000126e0, {0x400007c808, 0xf, 0xffffbbe49068?})
	/app/internal/dispose/dispose.go:116 +0x10c
portbio/internal/cli.(*app).runDispose(0x40000cddc8, {0x400007f280, 0x1, 0x1})
	/app/internal/cli/app.go:393 +0x7d8
portbio/internal/cli.(*app).route(0x0?, {0x400007f270, 0x4000059df8?, 0x27f3e8?})
	/app/internal/cli/app.go:128 +0x2fc
portbio/internal/cli.Run({0x400007f260?, 0x547f80?, 0xe0?}, {0x39d040?, 0x400007f200?}, {0x39d040?, 0x400007f230?})
	/app/internal/cli/app.go:91 +0x68
portbio/internal/cli_test.run(0x40000ac680, {0x4000059f08, 0x2, 0x3c37e7?})
	/app/internal/cli/app_test.go:15 +0x100
portbio/internal/cli_test.TestCLIDisposeBatchSkipsUnregistered(0x40000ac680)
	/app/internal/cli/app_test.go:36 +0x60
testing.tRunner(0x40000ac680, 0x3461a8)
	/usr/local/go/src/testing/testing.go:1689 +0xec
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x318
FAIL	portbio/internal/cli	0.022s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

定向测试与全量回归在 linux/amd64、linux/arm64 双架构下均通过。
对未注册处置器的措施执行处置返回非 nil 错误，可通过 errors.Is 判定为 model.ErrHandlerMissing，且不发生 panic。
批量处置把这类记录计入 skipped 并继续处置其余记录：样例数据下处置 13 条、跳过 2 条，处置数与跳过数之和等于提交数。
单条处置未注册措施时 CLI 退出码为 6，HTTP 侧返回 422；已注册措施的处置结果与已处置数量保持不变。
处置报表能标出各措施是否已在本口岸注册，并正常产出汇总。
