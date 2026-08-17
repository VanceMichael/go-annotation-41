// Package model 定义口岸外来物种查验与生物安全处置平台的领域模型。
//
// 平台覆盖物种名录与风险分级、进境查验截获登记、查验队列排序与分页、
// 批量复核、实验室并发检测与疫情通报上报。
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Risk 表示外来物种风险等级。
type Risk string

const (
	// RiskLow 低风险。
	RiskLow Risk = "low"
	// RiskMedium 中风险。
	RiskMedium Risk = "medium"
	// RiskHigh 高风险。
	RiskHigh Risk = "high"
	// RiskQuarantine 检疫性有害生物，最高等级。
	RiskQuarantine Risk = "quarantine"
)

// AllRisks 按由低到高的顺序返回全部风险等级。
func AllRisks() []Risk {
	return []Risk{RiskLow, RiskMedium, RiskHigh, RiskQuarantine}
}

// Rank 返回风险等级序号，数值越大风险越高。
func (r Risk) Rank() int {
	for i, v := range AllRisks() {
		if v == r {
			return i
		}
	}
	return -1
}

// DisplayName 返回风险等级中文名。
func (r Risk) DisplayName() string {
	switch r {
	case RiskLow:
		return "低风险"
	case RiskMedium:
		return "中风险"
	case RiskHigh:
		return "高风险"
	case RiskQuarantine:
		return "检疫性有害生物"
	default:
		return string(r)
	}
}

// ParseRisk 解析风险等级代码。
func ParseRisk(s string) (Risk, error) {
	v := Risk(strings.ToLower(strings.TrimSpace(s)))
	for _, r := range AllRisks() {
		if v == r {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownRisk, s)
}

// Measure 表示处置措施。
type Measure string

const (
	// MeasureRelease 放行。
	MeasureRelease Measure = "release"
	// MeasureDisinfect 消杀处理后放行。
	MeasureDisinfect Measure = "disinfect"
	// MeasureReturn 退运出境。
	MeasureReturn Measure = "return"
	// MeasureDestroy 销毁。
	MeasureDestroy Measure = "destroy"
	// MeasureLabHold 送实验室隔离检测，需上级授权。
	MeasureLabHold Measure = "lab-hold"
)

// AllMeasures 返回全部处置措施。
func AllMeasures() []Measure {
	return []Measure{MeasureRelease, MeasureDisinfect, MeasureReturn, MeasureDestroy, MeasureLabHold}
}

// DisplayName 返回处置措施中文名。
func (m Measure) DisplayName() string {
	switch m {
	case MeasureRelease:
		return "放行"
	case MeasureDisinfect:
		return "消杀后放行"
	case MeasureReturn:
		return "退运出境"
	case MeasureDestroy:
		return "销毁"
	case MeasureLabHold:
		return "送实验室隔离检测"
	default:
		return string(m)
	}
}

// ParseMeasure 解析处置措施代码。
func ParseMeasure(s string) (Measure, error) {
	v := Measure(strings.ToLower(strings.TrimSpace(s)))
	for _, m := range AllMeasures() {
		if v == m {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownMeasure, s)
}

// PermittedFor 报告该处置措施是否允许用于给定风险等级。
//
// 检疫性有害生物不得放行；高风险及以上不得仅消杀后放行。
func (m Measure) PermittedFor(r Risk) bool {
	switch m {
	case MeasureRelease:
		return r.Rank() <= RiskMedium.Rank()
	case MeasureDisinfect:
		return r.Rank() <= RiskHigh.Rank()
	default:
		return true
	}
}

// Channel 表示进境渠道。
type Channel string

const (
	// ChannelMail 国际邮件。
	ChannelMail Channel = "mail"
	// ChannelExpress 快件。
	ChannelExpress Channel = "express"
	// ChannelBaggage 旅客行李。
	ChannelBaggage Channel = "baggage"
	// ChannelCargo 货运。
	ChannelCargo Channel = "cargo"
)

// AllChannels 返回全部进境渠道。
func AllChannels() []Channel {
	return []Channel{ChannelMail, ChannelExpress, ChannelBaggage, ChannelCargo}
}

// DisplayName 返回进境渠道中文名。
func (c Channel) DisplayName() string {
	switch c {
	case ChannelMail:
		return "国际邮件"
	case ChannelExpress:
		return "快件"
	case ChannelBaggage:
		return "旅客行李"
	case ChannelCargo:
		return "货运"
	default:
		return string(c)
	}
}

// ParseChannel 解析进境渠道代码。
func ParseChannel(s string) (Channel, error) {
	v := Channel(strings.ToLower(strings.TrimSpace(s)))
	for _, k := range AllChannels() {
		if v == k {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownChannel, s)
}

// Species 是物种名录中的一条。
type Species struct {
	// Code 是名录编号。
	Code string `json:"code"`
	// NameZH 与 NameLatin 分别是中文名与学名。
	NameZH    string `json:"name_zh"`
	NameLatin string `json:"name_latin"`
	Risk      Risk   `json:"risk"`
	// Category 是生物类别，如昆虫、鱼类、植物。
	Category string `json:"category"`
	// Quarantine 报告该物种是否列入检疫性有害生物名单。
	Quarantine bool `json:"quarantine"`
}

// Validate 校验名录条目。
func (s Species) Validate() error {
	if strings.TrimSpace(s.Code) == "" {
		return fmt.Errorf("%w: 名录编号为空", ErrInvalidSpecies)
	}
	if strings.TrimSpace(s.NameZH) == "" || strings.TrimSpace(s.NameLatin) == "" {
		return fmt.Errorf("%w: 物种 %s 缺少名称", ErrInvalidSpecies, s.Code)
	}
	if _, err := ParseRisk(string(s.Risk)); err != nil {
		return fmt.Errorf("%w: 物种 %s 风险等级非法", ErrInvalidSpecies, s.Code)
	}
	if s.Quarantine && s.Risk != RiskQuarantine {
		return fmt.Errorf("%w: 物种 %s 列入检疫名单但风险等级不是检疫性有害生物",
			ErrInvalidSpecies, s.Code)
	}
	return nil
}

// Interception 是一条进境查验截获记录。
type Interception struct {
	ID string `json:"id"`
	// PortCode 是口岸代码。
	PortCode string `json:"port_code"`
	// SpeciesCode 是截获物种的名录编号。
	SpeciesCode string  `json:"species_code"`
	Channel     Channel `json:"channel"`
	Risk        Risk    `json:"risk"`
	Measure     Measure `json:"measure"`
	// Quantity 是截获数量。
	Quantity int `json:"quantity"`
	// AcceptedAt 是受理时间，用于查验队列排序。
	AcceptedAt time.Time `json:"accepted_at"`
}

// Validate 校验截获记录。
func (i Interception) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("%w: 截获编号为空", ErrInvalidInterception)
	}
	if strings.TrimSpace(i.PortCode) == "" {
		return fmt.Errorf("%w: 截获 %s 缺少口岸代码", ErrInvalidInterception, i.ID)
	}
	if strings.TrimSpace(i.SpeciesCode) == "" {
		return fmt.Errorf("%w: 截获 %s 缺少物种编号", ErrInvalidInterception, i.ID)
	}
	if _, err := ParseChannel(string(i.Channel)); err != nil {
		return fmt.Errorf("%w: 截获 %s 进境渠道非法", ErrInvalidInterception, i.ID)
	}
	if _, err := ParseRisk(string(i.Risk)); err != nil {
		return fmt.Errorf("%w: 截获 %s 风险等级非法", ErrInvalidInterception, i.ID)
	}
	if _, err := ParseMeasure(string(i.Measure)); err != nil {
		return fmt.Errorf("%w: 截获 %s 处置措施非法", ErrInvalidInterception, i.ID)
	}
	if i.Quantity <= 0 {
		return fmt.Errorf("%w: 截获 %s 数量必须为正", ErrInvalidInterception, i.ID)
	}
	if i.AcceptedAt.IsZero() {
		return fmt.Errorf("%w: 截获 %s 缺少受理时间", ErrInvalidInterception, i.ID)
	}
	return nil
}

// Sample 是送实验室检测的样本。
type Sample struct {
	ID string `json:"id"`
	// InterceptionID 是来源截获记录编号。
	InterceptionID string `json:"interception_id"`
	// Priority 报告该样本是否为加急样本。
	Priority bool `json:"priority"`
	// AssayMS 是检测耗时，单位毫秒，仅用于本地演练。
	AssayMS int `json:"assay_ms"`
}

// Validate 校验送检样本。
func (s Sample) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: 样本编号为空", ErrInvalidSample)
	}
	if strings.TrimSpace(s.InterceptionID) == "" {
		return fmt.Errorf("%w: 样本 %s 缺少来源截获记录", ErrInvalidSample, s.ID)
	}
	if s.AssayMS < 0 {
		return fmt.Errorf("%w: 样本 %s 检测耗时不得为负", ErrInvalidSample, s.ID)
	}
	return nil
}

// SortInterceptions 按「风险等级降序、受理时间升序、编号升序」稳定排序。
//
// 这是查验队列的唯一排序口径：风险高的先查，同风险先到先查。
func SortInterceptions(items []Interception) {
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].Risk != items[b].Risk {
			return items[a].Risk.Rank() > items[b].Risk.Rank()
		}
		if !items[a].AcceptedAt.Equal(items[b].AcceptedAt) {
			return items[a].AcceptedAt.Before(items[b].AcceptedAt)
		}
		return items[a].ID < items[b].ID
	})
}

// SortSpecies 按名录编号排序。
func SortSpecies(items []Species) {
	sort.SliceStable(items, func(a, b int) bool { return items[a].Code < items[b].Code })
}
