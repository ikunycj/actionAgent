package tools

import (
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
