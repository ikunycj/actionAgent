package tools

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAllowOnceTokenReplayRejected(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Tool{Name: "danger", Tier: L2, Run: func(in map[string]any) (map[string]any, error) { return in, nil }})
	mgr := NewApprovalManager()
	rt := NewRuntime(reg, mgr)
	binding := Binding{DeviceID: "d1", NodeID: "n1", RunID: "r1", Scope: "danger"}
	mgr.Issue(Token{ID: "tok1", Mode: AllowOnce, Binding: binding, Expires: time.Now().Add(time.Minute)})

	if _, err := rt.Execute("danger", map[string]any{"cmd": "rm"}, binding, "tok1"); err != nil {
		t.Fatalf("first execution should pass: %v", err)
	}
	if _, err := rt.Execute("danger", map[string]any{"cmd": "rm"}, binding, "tok1"); err == nil {
		t.Fatalf("expected replay rejection")
	}
}

func TestBindingMismatchRejected(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Tool{Name: "danger", Tier: L2, Run: func(in map[string]any) (map[string]any, error) { return in, nil }})
	mgr := NewApprovalManager()
	rt := NewRuntime(reg, mgr)
	mgr.Issue(Token{ID: "tok2", Mode: AllowAlways, Binding: Binding{DeviceID: "d1", NodeID: "n1", RunID: "r1", Scope: "danger"}, Expires: time.Now().Add(time.Minute)})

	_, err := rt.Execute("danger", map[string]any{"cmd": "x"}, Binding{DeviceID: "d2", NodeID: "n1", RunID: "r1", Scope: "danger"}, "tok2")
	if err == nil {
		t.Fatal("expected binding mismatch rejection")
	}
}

func TestAuditPersistenceAndQuery(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "tools-state.json")
	reg := NewRegistry()
	reg.Register(Tool{Name: "echo", Tier: L0, Run: func(in map[string]any) (map[string]any, error) { return in, nil }})
	mgr := NewApprovalManager()
	rt := NewRuntimeWithState(reg, mgr, statePath)

	if _, err := rt.Execute("echo", map[string]any{"a": 1}, Binding{}, ""); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if _, err := rt.Execute("echo", map[string]any{"a": 2}, Binding{}, ""); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	queried := rt.QueryAudit(1, "echo", "allow")
	if len(queried) != 1 {
		t.Fatalf("expected 1 queried record, got %d", len(queried))
	}

	rt2 := NewRuntimeWithState(reg, mgr, statePath)
	all := rt2.Audit()
	if len(all) < 2 {
		t.Fatalf("expected persisted audit records, got %d", len(all))
	}
}

func TestApprovalTokenPersistence(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "tools-state.json")
	reg := NewRegistry()
	mgr := NewApprovalManager()
	rt := NewRuntimeWithState(reg, mgr, statePath)
	_ = rt
	binding := Binding{DeviceID: "d1", NodeID: "n1", RunID: "r1", Scope: "danger"}
	mgr.Issue(Token{ID: "tok-persist", Mode: AllowAlways, Binding: binding, Expires: time.Now().Add(time.Minute)})

	rt2 := NewRuntimeWithState(reg, NewApprovalManager(), statePath)
	tokens := rt2.ListApprovalTokens(10)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 persisted token, got %d", len(tokens))
	}
	if tokens[0].ID != "tok-persist" {
		t.Fatalf("unexpected token id: %s", tokens[0].ID)
	}
}
