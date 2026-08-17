// Package dispose 负责按处置措施执行截获物的处置。
package dispose

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"portbio/internal/model"
)

// Disposal 是一次处置的结果。
type Disposal struct {
	InterceptionID string        `json:"interception_id"`
	Measure        model.Measure `json:"measure"`
	// Handled 是本次处置的数量。
	Handled int `json:"handled"`
	// Note 是处置说明。
	Note string `json:"note"`
}

// Handler 是某类处置措施的执行器。
type Handler func(model.Interception) (Disposal, error)

// Service 按措施派发处置。
//
// 各口岸能执行的处置措施并不相同：未在本口岸注册处置器的措施
// 必须返回 model.ErrHandlerMissing，交由上级授权后再处理。
type Service struct {
	mu       sync.RWMutex
	portCode string
	handlers map[model.Measure]Handler
	done     []Disposal
}

// NewService 构造处置服务。
func NewService(portCode string) *Service {
	return &Service{
		portCode: portCode,
		handlers: make(map[model.Measure]Handler),
	}
}

// PortCode 返回口岸代码。
func (s *Service) PortCode() string {
	return s.portCode
}

// Register 注册某措施的处置器。
func (s *Service) Register(m model.Measure, h Handler) error {
	if _, err := model.ParseMeasure(string(m)); err != nil {
		return err
	}
	if h == nil {
		return fmt.Errorf("%w: 措施 %s 的处置器为空", model.ErrHandlerMissing, m)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[m] = h
	return nil
}

// Registered 返回已注册处置器的措施，按代码升序排列。
func (s *Service) Registered() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.handlers))
	for m, h := range s.handlers {
		if h != nil {
			out = append(out, string(m))
		}
	}
	sort.Strings(out)
	return out
}

// Supports 报告本口岸是否能执行该措施。
func (s *Service) Supports(m model.Measure) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.handlers[m]
	return ok && h != nil
}

// Dispose 对一条截获记录执行处置。
//
// 本口岸未注册该措施的处置器时返回 model.ErrHandlerMissing，
// 不得因为缺少处置器而崩溃。
func (s *Service) Dispose(it model.Interception) (Disposal, error) {
	if err := it.Validate(); err != nil {
		return Disposal{}, err
	}

	s.mu.RLock()
	h, ok := s.handlers[it.Measure]
	s.mu.RUnlock()

	if !ok || h == nil {
		return Disposal{}, fmt.Errorf("%w: 措施 %s 在本口岸未注册处置器",
			model.ErrHandlerMissing, it.Measure)
	}

	d, err := h(it)
	if err != nil {
		return Disposal{}, err
	}
	s.mu.Lock()
	s.done = append(s.done, d)
	s.mu.Unlock()
	return d, nil
}

// DisposeAll 依次处置一批截获记录。
//
// 未注册处置器的记录被跳过并计入 skipped，不中断整批处置。
func (s *Service) DisposeAll(items []model.Interception) (done []Disposal, skipped []string, err error) {
	done = make([]Disposal, 0, len(items))
	skipped = make([]string, 0, 2)
	for _, it := range items {
		d, derr := s.Dispose(it)
		if derr != nil {
			if isHandlerMissing(derr) {
				skipped = append(skipped, it.ID)
				continue
			}
			return done, skipped, derr
		}
		done = append(done, d)
	}
	return done, skipped, nil
}

func isHandlerMissing(err error) bool {
	return errors.Is(err, model.ErrHandlerMissing)
}

// Done 返回已完成的处置记录副本。
func (s *Service) Done() []Disposal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Disposal, len(s.done))
	copy(out, s.done)
	return out
}

// HandledQuantity 返回已处置的总数量。
func (s *Service) HandledQuantity() int {
	n := 0
	for _, d := range s.Done() {
		n += d.Handled
	}
	return n
}

// Describe 返回处置结果的单行描述。
func Describe(d Disposal) string {
	return fmt.Sprintf("%s %s 数量%d %s",
		d.InterceptionID, d.Measure.DisplayName(), d.Handled, d.Note)
}
