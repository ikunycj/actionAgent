package dispatch

import (
	"strings"
	"testing"
	"time"

	"actionagent/agent/internal/core/task"
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

func TestHeartbeatStalenessAndRecovery(t *testing.T) {
	now := time.Now()
	d := NewWithOptions([]Node{
		{ID: "node-local", Local: true, Healthy: true, LastHeartbeat: now.Add(-10 * time.Second), Capabilities: map[string]bool{"run": true}},
		{ID: "node-remote", Local: false, Healthy: true, LastHeartbeat: now, Capabilities: map[string]bool{"run": true}},
	}, Options{
		HeartbeatTTL:  2 * time.Second,
		RelayDebounce: 200 * time.Millisecond,
	})

	n, err := d.LocalFirst(TaskRequirement{Capability: "run"})
	if err != nil {
		t.Fatalf("local first failed: %v", err)
	}
	if n.ID != "node-remote" {
		t.Fatalf("expected stale local to be excluded, got %s", n.ID)
	}

	changed := d.MarkStale(now)
	if changed == 0 {
		t.Fatal("expected stale node to be marked unhealthy")
	}

	if ok := d.Heartbeat("node-local", now); !ok {
		t.Fatal("expected heartbeat update success")
	}
	n, err = d.LocalFirst(TaskRequirement{Capability: "run"})
	if err != nil {
		t.Fatalf("local first failed after heartbeat: %v", err)
	}
	if n.ID != "node-local" {
		t.Fatalf("expected recovered local node, got %s", n.ID)
	}
}

func TestRelayDebounceAndSnapshotRecovery(t *testing.T) {
	now := time.Now()
	d := NewWithOptions([]Node{
		{ID: "node-a", Local: true, Healthy: true, LastHeartbeat: now, Capabilities: map[string]bool{"run": true}},
		{ID: "node-b", Local: false, Healthy: true, LastHeartbeat: now, Capabilities: map[string]bool{"run": true}},
	}, Options{
		HeartbeatTTL:  5 * time.Second,
		RelayDebounce: 500 * time.Millisecond,
	})
	src := Snapshot{TaskID: "task-1", RunID: "run-1", Attempt: 1, FromNode: "node-a"}
	n, snap, err := d.Relay(src, "run")
	if err != nil {
		t.Fatalf("relay failed: %v", err)
	}
	if n.ID != "node-b" || snap.Attempt != 2 {
		t.Fatalf("unexpected relay result: %+v %+v", n, snap)
	}
	recovered, ok := d.RecoverSnapshot("task-1")
	if !ok {
		t.Fatal("expected snapshot recovery success")
	}
	if recovered.FromNode != "node-b" || recovered.Attempt != 2 {
		t.Fatalf("unexpected recovered snapshot: %+v", recovered)
	}

	_, _, err = d.Relay(snap, "run")
	if err == nil || !strings.Contains(err.Error(), "debounce") {
		t.Fatalf("expected relay debounce error, got %v", err)
	}
}
