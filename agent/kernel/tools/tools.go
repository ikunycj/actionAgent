package tools

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type RiskTier string

const (
	L0 RiskTier = "L0"
	L1 RiskTier = "L1"
	L2 RiskTier = "L2"
)

type Tool struct {
	Name string
	Tier RiskTier
	Run  func(map[string]any) (map[string]any, error)
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry { return &Registry{tools: map[string]Tool{}} }

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	r.tools[t.Name] = t
	r.mu.Unlock()
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

type ApprovalMode string

const (
	AllowOnce   ApprovalMode = "allow-once"
	AllowAlways ApprovalMode = "allow-always"
)

type Binding struct {
	DeviceID string
	NodeID   string
	RunID    string
	Scope    string
}

type Token struct {
	ID       string
	Mode     ApprovalMode
	Binding  Binding
	Expires  time.Time
	Consumed bool
}

type ApprovalManager struct {
	mu     sync.Mutex
	tokens map[string]Token
}

func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{tokens: map[string]Token{}}
}

func (a *ApprovalManager) Issue(t Token) {
	a.mu.Lock()
	a.tokens[t.ID] = t
	a.mu.Unlock()
}

func (a *ApprovalManager) ValidateAndConsume(tokenID string, b Binding, now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.tokens[tokenID]
	if !ok {
		return errors.New("approval token not found")
	}
	if now.After(t.Expires) {
		return errors.New("approval token expired")
	}
	if t.Binding != b {
		return errors.New("approval binding mismatch")
	}
	if t.Mode == AllowOnce {
		if t.Consumed {
			return errors.New("approval token already consumed")
		}
		t.Consumed = true
		a.tokens[tokenID] = t
	}
	return nil
}

type AuditRecord struct {
	Timestamp time.Time      `json:"timestamp"`
	Tool      string         `json:"tool"`
	Tier      RiskTier       `json:"tier"`
	Decision  string         `json:"decision"`
	TokenID   string         `json:"token_id,omitempty"`
	Approver  string         `json:"approver,omitempty"`
	Command   string         `json:"command"`
	Error     string         `json:"error,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
}

type Runtime struct {
	registry  *Registry
	approvals *ApprovalManager
	mu        sync.Mutex
	audit     []AuditRecord
}

func NewRuntime(reg *Registry, approvals *ApprovalManager) *Runtime {
	if reg == nil {
		reg = NewRegistry()
	}
	if approvals == nil {
		approvals = NewApprovalManager()
	}
	return &Runtime{registry: reg, approvals: approvals}
}

func (r *Runtime) Execute(toolName string, input map[string]any, binding Binding, tokenID string) (map[string]any, error) {
	t, ok := r.registry.Get(toolName)
	if !ok {
		r.record(AuditRecord{Timestamp: time.Now().UTC(), Tool: toolName, Decision: "deny", Error: "tool_not_found"})
		return nil, errors.New("tool not found")
	}
	if t.Tier == L2 {
		if err := r.approvals.ValidateAndConsume(tokenID, binding, time.Now()); err != nil {
			r.record(AuditRecord{Timestamp: time.Now().UTC(), Tool: toolName, Tier: t.Tier, Decision: "deny", TokenID: tokenID, Command: fmt.Sprint(input), Error: err.Error()})
			return nil, err
		}
	}
	res, err := t.Run(input)
	rec := AuditRecord{Timestamp: time.Now().UTC(), Tool: toolName, Tier: t.Tier, Decision: "allow", TokenID: tokenID, Command: fmt.Sprint(input), Result: res}
	if err != nil {
		rec.Error = err.Error()
		r.record(rec)
		return nil, err
	}
	r.record(rec)
	return res, nil
}

func (r *Runtime) record(a AuditRecord) {
	r.mu.Lock()
	r.audit = append(r.audit, a)
	r.mu.Unlock()
}

func (r *Runtime) Audit() []AuditRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AuditRecord, len(r.audit))
	copy(out, r.audit)
	return out
}
