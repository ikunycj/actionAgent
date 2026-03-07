package dispatch

import (
	"testing"
	"time"

	"actionagent/agent/kernel/task"
)

func TestLocalFirstSelection(t *testing.T) {
	d := New([]Node{
		{ID: "local", Local: true, Healthy: true, Capabilities: map[string]bool{"run": true}},
		{ID: "remote", Local: false, Healthy: true, Capabilities: map[string]bool{"run": true}},
	})
	n, err := d.LocalFirst(TaskRequirement{Capability: "run"})
	if err != nil {
		t.Fatalf("local first failed: %v", err)
	}
	if n.ID != "local" {
		t.Fatalf("expected local, got %s", n.ID)
	}
}

func TestRelayAndConvergence(t *testing.T) {
	d := New([]Node{
		{ID: "node-a", Local: true, Healthy: false, Capabilities: map[string]bool{"run": true}},
		{ID: "node-b", Local: false, Healthy: true, Capabilities: map[string]bool{"run": true}},
	})
	n, snap, err := d.Relay(Snapshot{TaskID: "t1", RunID: "r1", Attempt: 1, FromNode: "node-a"}, "run")
	if err != nil {
		t.Fatalf("relay failed: %v", err)
	}
	if n.ID != "node-b" || snap.Attempt != 2 {
		t.Fatalf("unexpected relay result: node=%s attempt=%d", n.ID, snap.Attempt)
	}

	agg := NewTerminalAggregator()
	first := agg.Record(task.Outcome{TaskID: "t1", RunID: "r1", State: task.StateSucceeded, FinishedAt: time.Now()})
	second := agg.Record(task.Outcome{TaskID: "t1", RunID: "r2", State: task.StateFailed, FinishedAt: time.Now().Add(time.Second)})
	if first.RunID != second.RunID {
		t.Fatalf("expected convergence to single terminal run, got %s and %s", first.RunID, second.RunID)
	}
}
