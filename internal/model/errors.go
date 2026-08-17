package model

import "errors"

// 哨兵错误。调用方使用 errors.Is 沿错误链判定，不要比较错误文本。
var (
	// ErrUnknownRisk 表示风险等级代码无法识别。
	ErrUnknownRisk = errors.New("model: 未知风险等级")
	// ErrUnknownMeasure 表示处置措施代码无法识别。
	ErrUnknownMeasure = errors.New("model: 未知处置措施")
	// ErrUnknownChannel 表示进境渠道代码无法识别。
	ErrUnknownChannel = errors.New("model: 未知进境渠道")

	// ErrInvalidSpecies 表示物种名录条目非法。
	ErrInvalidSpecies = errors.New("model: 物种名录条目非法")
	// ErrInvalidInterception 表示截获记录非法。
	ErrInvalidInterception = errors.New("model: 截获记录非法")
	// ErrInvalidSample 表示送检样本非法。
	ErrInvalidSample = errors.New("model: 送检样本非法")

	// ErrSpeciesNotFound 表示物种不在名录中。
	ErrSpeciesNotFound = errors.New("model: 物种不在名录中")
	// ErrInterceptionNotFound 表示截获记录不存在。
	ErrInterceptionNotFound = errors.New("model: 截获记录不存在")

	// ErrHandlerMissing 表示该处置措施在本口岸未注册处置器。
	ErrHandlerMissing = errors.New("model: 处置措施未注册处置器")
	// ErrMeasureNotPermitted 表示该风险等级不允许使用该处置措施。
	ErrMeasureNotPermitted = errors.New("model: 处置措施与风险等级不匹配")

	// ErrQueuePagination 表示查验队列分页结果不完整。
	ErrQueuePagination = errors.New("model: 查验队列分页不完整")
	// ErrSweepAborted 表示批量复核被调用方中止。
	ErrSweepAborted = errors.New("model: 批量复核被中止")
	// ErrLabBatchFailed 表示实验室批次检测失败。
	ErrLabBatchFailed = errors.New("model: 实验室批次检测失败")
	// ErrLabResultMissing 表示实验室结果与送检样本数不符。
	ErrLabResultMissing = errors.New("model: 实验室结果与送检样本不符")

	// ErrNotifyThrottled 表示通报通道调用频次超限，属于可重试错误。
	ErrNotifyThrottled = errors.New("notify: 通报通道调用频次超限")
	// ErrNotifyChannelDisabled 表示通报通道已停用，属于确定性错误，不应重试。
	ErrNotifyChannelDisabled = errors.New("notify: 通报通道已停用")
	// ErrNotifyRejected 表示通报内容被上级系统拒收，属于确定性错误。
	ErrNotifyRejected = errors.New("notify: 通报内容被拒收")
)
