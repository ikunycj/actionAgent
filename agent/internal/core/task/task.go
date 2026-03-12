package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateCreated         State = "CREATED"
	StateQueued          State = "QUEUED"
	StateDispatching     State = "DISPATCHING"
	StateRunning         State = "RUNNING"
	StateWaitingApproval State = "WAITING_APPROVAL"
	StateRetrying        State = "RETRYING"
	StateSuspended       State = "SUSPENDED"
	StateSucceeded       State = "SUCCEEDED"
	StateFailed          State = "FAILED"
	StateCancelled       State = "CANCELLED"
)

var legalTransitions = map[State]map[State]struct{}{
	StateCreated:         {StateQueued: {}},
	StateQueued:          {StateDispatching: {}, StateCancelled: {}},
	StateDispatching:     {StateRunning: {}, StateRetrying: {}, StateFailed: {}, StateCancelled: {}},
	StateRunning:         {StateSucceeded: {}, StateFailed: {}, StateCancelled: {}, StateWaitingApproval: {}, StateRetrying: {}, StateSuspended: {}},
	StateWaitingApproval: {StateRunning: {}, StateFailed: {}, StateCancelled: {}},
	StateRetrying:        {StateRunning: {}, StateFailed: {}, StateCancelled: {}},
	StateSuspended:       {StateRunning: {}, StateCancelled: {}},
}

func CanTransition(from, to State) bool {
	if from == to {
		return true
	}
	next, ok := legalTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

type ExecutionEnvelope struct {
	RequestID      string         `json:"request_id"`
	TaskID         string         `json:"task_id"`
	RunID          string         `json:"run_id"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	Lane           string         `json:"lane"`
	SessionKey     string         `json:"session_key,omitempty"`
	Operation      string         `json:"operation"`
	Input          map[string]any `json:"input"`
	CreatedAt      time.Time      `json:"created_at"`
	Attempt        int            `json:"attempt"`
	TimeoutMillis  int            `json:"timeout_ms"`
}

type Outcome struct {
	TaskID     string         `json:"task_id"`
	RunID      string         `json:"run_id"`
	AgentID    string         `json:"agent_id,omitempty"`
	State      State          `json:"state"`
	NodeID     string         `json:"node_id,omitempty"`
	Error      string         `json:"error,omitempty"`
	Replay     bool           `json:"replay"`
	Payload    map[string]any `json:"payload,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
}

type NodeChoice struct {
	NodeID string
	Relay  bool
}

type Dispatcher interface {
	Choose(context.Context, ExecutionEnvelope) (NodeChoice, error)
}

type Executor interface {
	Execute(context.Context, ExecutionEnvelope) (map[string]any, error)
}

type Engine struct {
	queue      *LaneQueue
	dedupe     *DedupeStore
	dispatcher Dispatcher
	executor   Executor
	ttl        time.Duration
}

func NewEngine(queue *LaneQueue, dedupe *DedupeStore, dispatcher Dispatcher, executor Executor, ttl time.Duration) *Engine {
	if queue == nil {
		queue = NewLaneQueue(4)
	}
	if dedupe == nil {
		dedupe = NewDedupeStore()
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Engine{queue: queue, dedupe: dedupe, dispatcher: dispatcher, executor: executor, ttl: ttl}
}

func (e *Engine) EnterDraining() {
	e.queue.SetDraining(true)
}

func (e *Engine) ExitDraining() {
	e.queue.SetDraining(false)
}

func (e *Engine) Pending() int {
	if e == nil || e.queue == nil {
		return 0
	}
	return e.queue.Pending()
}

func (e *Engine) Submit(ctx context.Context, env ExecutionEnvelope) (Outcome, error) {
	if env.CreatedAt.IsZero() {
		env.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(env.Lane) == "" {
		env.Lane = "main"
	}
	if env.TimeoutMillis <= 0 {
		env.TimeoutMillis = 30000
	}

	if env.IdempotencyKey != "" {
		if out, ok := e.dedupe.Get(env.IdempotencyKey, time.Now()); ok {
			out.Replay = true
			return out, nil
		}
		if !e.dedupe.SetPending(env.IdempotencyKey, time.Now().Add(e.ttl)) {
			if out, ok := e.dedupe.Get(env.IdempotencyKey, time.Now()); ok {
				out.Replay = true
				return out, nil
			}
		}
	}

	var result Outcome
	err := e.queue.Submit(ctx, env.Lane, func() error {
		started := time.Now().UTC()
		node := NodeChoice{NodeID: "local"}
		if e.dispatcher != nil {
			c, err := e.dispatcher.Choose(ctx, env)
			if err != nil {
				result = Outcome{TaskID: env.TaskID, RunID: env.RunID, State: StateFailed, Error: err.Error(), StartedAt: started, FinishedAt: time.Now().UTC()}
				return nil
			}
			node = c
		}

		payload := map[string]any{"accepted": true, "operation": env.Operation}
		state := StateSucceeded
		if e.executor != nil {
			out, err := e.executor.Execute(ctx, env)
			if err != nil {
				state = StateFailed
				payload = map[string]any{"error": err.Error()}
			} else if out != nil {
				payload = out
			}
		}

		result = Outcome{
			TaskID:     env.TaskID,
			RunID:      env.RunID,
			AgentID:    env.AgentID,
			State:      state,
			NodeID:     node.NodeID,
			Payload:    payload,
			StartedAt:  started,
			FinishedAt: time.Now().UTC(),
		}
		if state == StateFailed {
			if msg, _ := payload["error"].(string); msg != "" {
				result.Error = msg
			}
		}
		return nil
	})
	if err != nil {
		return Outcome{}, err
	}
	if env.IdempotencyKey != "" {
		e.dedupe.Complete(env.IdempotencyKey, result, time.Now().Add(e.ttl))
	}
	return result, nil
}

type LaneQueue struct {
	mu                sync.Mutex
	sems              map[string]chan struct{}
	defaultConcurrent int
	draining          bool
	pending           int
}

func NewLaneQueue(defaultConcurrent int) *LaneQueue {
	if defaultConcurrent < 1 {
		defaultConcurrent = 1
	}
	return &LaneQueue{sems: map[string]chan struct{}{}, defaultConcurrent: defaultConcurrent}
}

func (q *LaneQueue) capacityForLane(lane string) int {
	if strings.HasPrefix(lane, "session:") {
		return 1
	}
	return q.defaultConcurrent
}

func (q *LaneQueue) semaphore(lane string) chan struct{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ch, ok := q.sems[lane]; ok {
		return ch
	}
	ch := make(chan struct{}, q.capacityForLane(lane))
	q.sems[lane] = ch
	return ch
}

func (q *LaneQueue) Submit(ctx context.Context, lane string, fn func() error) error {
	if lane == "" {
		lane = "main"
	}
	q.mu.Lock()
	if q.draining {
		q.mu.Unlock()
		return errors.New("runtime is draining")
	}
	q.pending++
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		q.pending--
		q.mu.Unlock()
	}()

	sem := q.semaphore(lane)
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if fn == nil {
		return nil
	}
	return fn()
}

func (q *LaneQueue) SetDraining(v bool) {
	q.mu.Lock()
	q.draining = v
	q.mu.Unlock()
}

func (q *LaneQueue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending
}

type dedupeEntry struct {
	pending bool
	expires time.Time
	outcome Outcome
}

type DedupeStore struct {
	mu      sync.Mutex
	entries map[string]dedupeEntry
}

func NewDedupeStore() *DedupeStore {
	return &DedupeStore{entries: map[string]dedupeEntry{}}
}

func (d *DedupeStore) SetPending(key string, expires time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.entries[key]; ok && e.expires.After(time.Now()) {
		return false
	}
	d.entries[key] = dedupeEntry{pending: true, expires: expires}
	return true
}

func (d *DedupeStore) Complete(key string, out Outcome, expires time.Time) {
	d.mu.Lock()
	d.entries[key] = dedupeEntry{pending: false, expires: expires, outcome: out}
	d.mu.Unlock()
}

func (d *DedupeStore) Get(key string, now time.Time) (Outcome, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[key]
	if !ok {
		return Outcome{}, false
	}
	if now.After(e.expires) {
		delete(d.entries, key)
		return Outcome{}, false
	}
	if e.pending {
		return Outcome{}, false
	}
	return e.outcome, true
}

func (d *DedupeStore) Cleanup(now time.Time) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	removed := 0
	for k, v := range d.entries {
		if now.After(v.expires) {
			delete(d.entries, k)
			removed++
		}
	}
	return removed
}

func ValidateTransitionSequence(seq []State) error {
	if len(seq) < 2 {
		return nil
	}
	for i := 0; i < len(seq)-1; i++ {
		if !CanTransition(seq[i], seq[i+1]) {
			return fmt.Errorf("illegal transition: %s -> %s", seq[i], seq[i+1])
		}
	}
	return nil
}
