package dispatch

import (
	"context"
	"errors"
	"sync"
	"time"

	"actionagent/agent/kernel/task"
)

type Node struct {
	ID            string
	Local         bool
	Healthy       bool
	Capabilities  map[string]bool
	LastHeartbeat time.Time
}

type TaskRequirement struct {
	Capability string
}

type Snapshot struct {
	TaskID   string         `json:"task_id"`
	RunID    string         `json:"run_id"`
	Attempt  int            `json:"attempt"`
	Input    map[string]any `json:"input"`
	FromNode string         `json:"from_node"`
}

type Dispatcher struct {
	mu    sync.RWMutex
	nodes []Node
}

func New(nodes []Node) *Dispatcher {
	return &Dispatcher{nodes: nodes}
}

func (d *Dispatcher) SetNodes(nodes []Node) {
	d.mu.Lock()
	d.nodes = nodes
	d.mu.Unlock()
}

func (d *Dispatcher) Choose(_ context.Context, env task.ExecutionEnvelope) (task.NodeChoice, error) {
	req := TaskRequirement{Capability: env.Operation}
	n, err := d.LocalFirst(req)
	if err != nil {
		return task.NodeChoice{}, err
	}
	return task.NodeChoice{NodeID: n.ID}, nil
}

func (d *Dispatcher) LocalFirst(req TaskRequirement) (Node, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, n := range d.nodes {
		if n.Local && n.Healthy && supports(n, req.Capability) {
			return n, nil
		}
	}
	for _, n := range d.nodes {
		if !n.Local && n.Healthy && supports(n, req.Capability) {
			return n, nil
		}
	}
	return Node{}, errors.New("no eligible node")
}

func (d *Dispatcher) Relay(snapshot Snapshot, capability string) (Node, Snapshot, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, n := range d.nodes {
		if n.Healthy && n.ID != snapshot.FromNode && supports(n, capability) {
			ns := snapshot
			ns.Attempt++
			ns.FromNode = n.ID
			return n, ns, nil
		}
	}
	return Node{}, Snapshot{}, errors.New("no relay candidate")
}

func supports(n Node, capability string) bool {
	if len(n.Capabilities) == 0 {
		return true
	}
	if capability == "" {
		return true
	}
	return n.Capabilities[capability]
}

type TerminalAggregator struct {
	mu       sync.Mutex
	terminal map[string]task.Outcome
}

func NewTerminalAggregator() *TerminalAggregator {
	return &TerminalAggregator{terminal: map[string]task.Outcome{}}
}

func (a *TerminalAggregator) Record(out task.Outcome) task.Outcome {
	a.mu.Lock()
	defer a.mu.Unlock()
	if existing, ok := a.terminal[out.TaskID]; ok {
		return existing
	}
	a.terminal[out.TaskID] = out
	return out
}

func (a *TerminalAggregator) Get(taskID string) (task.Outcome, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	out, ok := a.terminal[taskID]
	return out, ok
}
