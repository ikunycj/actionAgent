package dispatch

import (
	"context"
	"errors"
	"sort"
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

type Options struct {
	HeartbeatTTL  time.Duration
	RelayDebounce time.Duration
}

func DefaultOptions() Options {
	return Options{
		HeartbeatTTL:  15 * time.Second,
		RelayDebounce: 500 * time.Millisecond,
	}
}

type Dispatcher struct {
	mu        sync.RWMutex
	nodes     []Node
	opts      Options
	lastRelay map[string]time.Time
	snapshots map[string]Snapshot
}

func New(nodes []Node) *Dispatcher {
	return NewWithOptions(nodes, DefaultOptions())
}

func NewWithOptions(nodes []Node, opts Options) *Dispatcher {
	if opts.HeartbeatTTL <= 0 {
		opts.HeartbeatTTL = 15 * time.Second
	}
	if opts.RelayDebounce <= 0 {
		opts.RelayDebounce = 500 * time.Millisecond
	}
	return &Dispatcher{
		nodes:     nodes,
		opts:      opts,
		lastRelay: map[string]time.Time{},
		snapshots: map[string]Snapshot{},
	}
}

func (d *Dispatcher) SetNodes(nodes []Node) {
	d.mu.Lock()
	d.nodes = nodes
	d.mu.Unlock()
}

func (d *Dispatcher) OnlineCount(now time.Time) int {
	if now.IsZero() {
		now = time.Now()
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := 0
	for _, node := range d.nodes {
		if d.isHealthy(node, now) {
			n++
		}
	}
	return n
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
	now := time.Now()
	for _, n := range d.nodes {
		if n.Local && d.isHealthy(n, now) && supports(n, req.Capability) {
			return n, nil
		}
	}
	for _, n := range d.nodes {
		if !n.Local && d.isHealthy(n, now) && supports(n, req.Capability) {
			return n, nil
		}
	}
	return Node{}, errors.New("no eligible node")
}

func (d *Dispatcher) Relay(snapshot Snapshot, capability string) (Node, Snapshot, error) {
	if snapshot.TaskID == "" {
		return Node{}, Snapshot{}, errors.New("task_id is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if last, ok := d.lastRelay[snapshot.TaskID]; ok && now.Sub(last) < d.opts.RelayDebounce {
		return Node{}, Snapshot{}, errors.New("relay debounce active")
	}
	for _, n := range d.nodes {
		if d.isHealthy(n, now) && n.ID != snapshot.FromNode && supports(n, capability) {
			ns := snapshot
			ns.Attempt++
			ns.FromNode = n.ID
			d.lastRelay[ns.TaskID] = now
			d.snapshots[ns.TaskID] = ns
			return n, ns, nil
		}
	}
	return Node{}, Snapshot{}, errors.New("no relay candidate")
}

func (d *Dispatcher) Heartbeat(nodeID string, at time.Time) bool {
	if at.IsZero() {
		at = time.Now()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.nodes {
		if d.nodes[i].ID == nodeID {
			d.nodes[i].LastHeartbeat = at
			d.nodes[i].Healthy = true
			return true
		}
	}
	return false
}

func (d *Dispatcher) MarkStale(now time.Time) int {
	if now.IsZero() {
		now = time.Now()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	changed := 0
	for i := range d.nodes {
		if d.isStale(d.nodes[i], now) && d.nodes[i].Healthy {
			d.nodes[i].Healthy = false
			changed++
		}
	}
	return changed
}

func (d *Dispatcher) SaveSnapshot(snapshot Snapshot) {
	if snapshot.TaskID == "" {
		return
	}
	d.mu.Lock()
	d.snapshots[snapshot.TaskID] = snapshot
	d.mu.Unlock()
}

func (d *Dispatcher) RecoverSnapshot(taskID string) (Snapshot, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.snapshots[taskID]
	return s, ok
}

func (d *Dispatcher) isHealthy(n Node, now time.Time) bool {
	return n.Healthy && !d.isStale(n, now)
}

func (d *Dispatcher) isStale(n Node, now time.Time) bool {
	if n.LastHeartbeat.IsZero() {
		return false
	}
	return now.Sub(n.LastHeartbeat) > d.opts.HeartbeatTTL
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

func (a *TerminalAggregator) List(limit int) []task.Outcome {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]task.Outcome, 0, len(a.terminal))
	for _, v := range a.terminal {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FinishedAt.After(out[j].FinishedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
