package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type staticDispatcher struct{}

func (staticDispatcher) Choose(context.Context, ExecutionEnvelope) (NodeChoice, error) {
	return NodeChoice{NodeID: "local"}, nil
}

type staticExecutor struct{}

func (staticExecutor) Execute(_ context.Context, env ExecutionEnvelope) (map[string]any, error) {
	return map[string]any{"operation": env.Operation}, nil
}

func TestTransitionValidation(t *testing.T) {
	ok := []State{StateCreated, StateQueued, StateDispatching, StateRunning, StateSucceeded}
	if err := ValidateTransitionSequence(ok); err != nil {
		t.Fatalf("valid sequence rejected: %v", err)
	}
	bad := []State{StateCreated, StateRunning}
	if err := ValidateTransitionSequence(bad); err == nil {
		t.Fatal("expected invalid transition failure")
	}
}

func TestQueueSessionSerialization(t *testing.T) {
	q := NewLaneQueue(4)
	ctx := context.Background()
	var active int64
	var maxActive int64
	wg := sync.WaitGroup{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := q.Submit(ctx, "session:abc", func() error {
				v := atomic.AddInt64(&active, 1)
				for {
					m := atomic.LoadInt64(&maxActive)
					if v <= m || atomic.CompareAndSwapInt64(&maxActive, m, v) {
						break
					}
				}
				time.Sleep(15 * time.Millisecond)
				atomic.AddInt64(&active, -1)
				return nil
			})
			if err != nil {
				t.Errorf("submit failed: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt64(&maxActive); got != 1 {
		t.Fatalf("expected serialized session lane, max active=%d", got)
	}
}

func TestDedupeReplayAndCleanup(t *testing.T) {
	e := NewEngine(NewLaneQueue(2), NewDedupeStore(), staticDispatcher{}, staticExecutor{}, time.Second)
	env := ExecutionEnvelope{TaskID: "t1", RunID: "r1", Operation: "run", Lane: "main", IdempotencyKey: "idem-key"}
	first, err := e.Submit(context.Background(), env)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	second, err := e.Submit(context.Background(), env)
	if err != nil {
		t.Fatalf("submit replay failed: %v", err)
	}
	if !second.Replay {
		t.Fatalf("expected replay result")
	}
	if first.TaskID != second.TaskID {
		t.Fatalf("expected same task id, got %s %s", first.TaskID, second.TaskID)
	}
	removed := e.dedupe.Cleanup(time.Now().Add(2 * time.Second))
	if removed == 0 {
		t.Fatalf("expected cleanup removed entries")
	}
}

func TestDrainingRejectsNewTasks(t *testing.T) {
	e := NewEngine(NewLaneQueue(1), NewDedupeStore(), staticDispatcher{}, staticExecutor{}, time.Second)
	e.EnterDraining()
	_, err := e.Submit(context.Background(), ExecutionEnvelope{TaskID: "t", RunID: "r", Lane: "main"})
	if err == nil {
		t.Fatal("expected draining rejection")
	}
}
