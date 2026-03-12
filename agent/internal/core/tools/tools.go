package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	ID       string       `json:"id"`
	Mode     ApprovalMode `json:"mode"`
	Binding  Binding      `json:"binding"`
	Expires  time.Time    `json:"expires"`
	Consumed bool         `json:"consumed"`
}

type ApprovalManager struct {
	mu       sync.Mutex
	tokens   map[string]Token
	onChange func()
}

func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{tokens: map[string]Token{}}
}

func (a *ApprovalManager) SetOnChange(fn func()) {
	a.mu.Lock()
	a.onChange = fn
	a.mu.Unlock()
}

func (a *ApprovalManager) notifyChange() {
	a.mu.Lock()
	fn := a.onChange
	a.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (a *ApprovalManager) Issue(t Token) {
	a.mu.Lock()
	a.tokens[t.ID] = t
	a.mu.Unlock()
	a.notifyChange()
}

func (a *ApprovalManager) ValidateAndConsume(tokenID string, b Binding, now time.Time) error {
	changed := false
	a.mu.Lock()
	t, ok := a.tokens[tokenID]
	if !ok {
		a.mu.Unlock()
		return errors.New("approval token not found")
	}
	if now.After(t.Expires) {
		a.mu.Unlock()
		return errors.New("approval token expired")
	}
	if t.Binding != b {
		a.mu.Unlock()
		return errors.New("approval binding mismatch")
	}
	if t.Mode == AllowOnce {
		if t.Consumed {
			a.mu.Unlock()
			return errors.New("approval token already consumed")
		}
		t.Consumed = true
		a.tokens[tokenID] = t
		changed = true
	}
	a.mu.Unlock()
	if changed {
		a.notifyChange()
	}
	return nil
}

func (a *ApprovalManager) Snapshot() map[string]Token {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]Token, len(a.tokens))
	for k, v := range a.tokens {
		out[k] = v
	}
	return out
}

func (a *ApprovalManager) Restore(tokens map[string]Token) {
	a.mu.Lock()
	a.tokens = make(map[string]Token, len(tokens))
	for k, v := range tokens {
		a.tokens[k] = v
	}
	a.mu.Unlock()
}

func (a *ApprovalManager) List(limit int) []Token {
	a.mu.Lock()
	out := make([]Token, 0, len(a.tokens))
	for _, t := range a.tokens {
		out = append(out, t)
	}
	a.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		return out[i].Expires.After(out[j].Expires)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
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

type persistedState struct {
	Tokens map[string]Token `json:"tokens"`
	Audit  []AuditRecord    `json:"audit"`
}

type Runtime struct {
	registry  *Registry
	approvals *ApprovalManager
	mu        sync.Mutex
	audit     []AuditRecord

	statePath string
	maxAudit  int
}

func NewRuntime(reg *Registry, approvals *ApprovalManager) *Runtime {
	return NewRuntimeWithState(reg, approvals, "")
}

func NewRuntimeWithState(reg *Registry, approvals *ApprovalManager, statePath string) *Runtime {
	if reg == nil {
		reg = NewRegistry()
	}
	if approvals == nil {
		approvals = NewApprovalManager()
	}
	r := &Runtime{registry: reg, approvals: approvals, statePath: statePath, maxAudit: 2000}
	r.loadState()
	r.approvals.SetOnChange(r.persistState)
	return r
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
	if r.maxAudit > 0 && len(r.audit) > r.maxAudit {
		r.audit = r.audit[len(r.audit)-r.maxAudit:]
	}
	r.mu.Unlock()
	r.persistState()
}

func (r *Runtime) Audit() []AuditRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AuditRecord, len(r.audit))
	copy(out, r.audit)
	return out
}

func (r *Runtime) QueryAudit(limit int, toolName, decision string) []AuditRecord {
	toolName = strings.TrimSpace(toolName)
	decision = strings.TrimSpace(decision)
	src := r.Audit()
	out := make([]AuditRecord, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- {
		rec := src[i]
		if toolName != "" && rec.Tool != toolName {
			continue
		}
		if decision != "" && rec.Decision != decision {
			continue
		}
		out = append(out, rec)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (r *Runtime) ListApprovalTokens(limit int) []Token {
	return r.approvals.List(limit)
}

func (r *Runtime) loadState() {
	if strings.TrimSpace(r.statePath) == "" {
		return
	}
	b, err := os.ReadFile(r.statePath)
	if err != nil {
		return
	}
	var state persistedState
	if err := json.Unmarshal(b, &state); err != nil {
		return
	}
	r.approvals.Restore(state.Tokens)
	r.mu.Lock()
	r.audit = make([]AuditRecord, len(state.Audit))
	copy(r.audit, state.Audit)
	if r.maxAudit > 0 && len(r.audit) > r.maxAudit {
		r.audit = r.audit[len(r.audit)-r.maxAudit:]
	}
	r.mu.Unlock()
}

func (r *Runtime) persistState() {
	if strings.TrimSpace(r.statePath) == "" {
		return
	}
	state := persistedState{
		Tokens: r.approvals.Snapshot(),
	}
	r.mu.Lock()
	state.Audit = make([]AuditRecord, len(r.audit))
	copy(state.Audit, r.audit)
	r.mu.Unlock()

	b, err := json.Marshal(state)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(r.statePath), 0o755); err != nil {
		return
	}
	tmp := r.statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, r.statePath)
}
