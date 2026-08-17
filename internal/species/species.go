// Package species 负责物种名录与风险分级查询。
package species

import (
	"fmt"
	"sort"
	"sync"

	"portbio/internal/model"
)

// Catalog 是物种名录。
type Catalog struct {
	mu    sync.RWMutex
	byID  map[string]model.Species
	order []string
}

// NewCatalog 构造物种名录。
func NewCatalog() *Catalog {
	return &Catalog{byID: make(map[string]model.Species)}
}

// Add 登记一条名录。
func (c *Catalog) Add(s model.Species) error {
	if err := s.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, dup := c.byID[s.Code]; !dup {
		c.order = append(c.order, s.Code)
	}
	c.byID[s.Code] = s
	return nil
}

// AddAll 批量登记名录。
func (c *Catalog) AddAll(items []model.Species) error {
	for _, s := range items {
		if err := c.Add(s); err != nil {
			return err
		}
	}
	return nil
}

// Get 按名录编号查询。
func (c *Catalog) Get(code string) (model.Species, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.byID[code]
	if !ok {
		return model.Species{}, fmt.Errorf("%w: %s", model.ErrSpeciesNotFound, code)
	}
	return s, nil
}

// List 按登记顺序返回全部名录。
func (c *Catalog) List() []model.Species {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Species, 0, len(c.byID))
	for _, code := range c.order {
		out = append(out, c.byID[code])
	}
	return out
}

// RiskOf 返回某物种的风险等级。
func (c *Catalog) RiskOf(code string) (model.Risk, error) {
	s, err := c.Get(code)
	if err != nil {
		return "", err
	}
	return s.Risk, nil
}

// QuarantineCodes 返回列入检疫性有害生物名单的物种编号，按升序排列。
func (c *Catalog) QuarantineCodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.byID))
	for code, s := range c.byID {
		if s.Quarantine {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}

// CountByRisk 按风险等级统计名录条数。
func (c *Catalog) CountByRisk() map[model.Risk]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[model.Risk]int)
	for _, r := range model.AllRisks() {
		out[r] = 0
	}
	for _, s := range c.byID {
		out[s.Risk]++
	}
	return out
}

// CheckMeasure 校验截获记录的处置措施是否与风险等级匹配。
func (c *Catalog) CheckMeasure(it model.Interception) error {
	if !it.Measure.PermittedFor(it.Risk) {
		return fmt.Errorf("%w: %s 不得用于%s",
			model.ErrMeasureNotPermitted, it.Measure.DisplayName(), it.Risk.DisplayName())
	}
	return nil
}

// Describe 返回名录条目的单行描述。
func Describe(s model.Species) string {
	flag := ""
	if s.Quarantine {
		flag = " [检疫名单]"
	}
	return fmt.Sprintf("%s %s（%s）%s %s%s",
		s.Code, s.NameZH, s.NameLatin, s.Category, s.Risk.DisplayName(), flag)
}
