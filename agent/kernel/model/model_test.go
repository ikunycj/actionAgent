package model

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type sequenceAdapter struct {
	provider string
	sleep    time.Duration

	mu      sync.Mutex
	results []error
	calls   int
}

func (a *sequenceAdapter) Name() string { return a.provider }

func (a *sequenceAdapter) Complete(ctx context.Context, req Request, _ Credential) (Response, error) {
	a.mu.Lock()
	a.calls++
	if a.sleep > 0 {
		a.mu.Unlock()
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-time.After(a.sleep):
		}
		a.mu.Lock()
	}
	var err error
	if len(a.results) > 0 {
		err = a.results[0]
		a.results = a.results[1:]
	}
	a.mu.Unlock()
	if err != nil {
		return Response{}, err
	}
	return Response{Provider: a.provider, Model: req.Model, Output: map[string]any{"ok": true}}, nil
}

func (a *sequenceAdapter) Calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

func TestRouterRetriesTransientFailure(t *testing.T) {
	pool := NewCredentialPool()
	pool.Set("primary", []Credential{{ID: "cred-1", Secret: "s"}})
	adapter := &sequenceAdapter{
		provider: "primary",
		results: []error{
			&ProviderError{Class: ErrTimeout, Msg: "temporary timeout"},
			nil,
		},
	}
	r := NewRouterWithOptions("primary", nil, pool, ResilienceOptions{
		ProviderTimeout:         500 * time.Millisecond,
		MaxAttempts:             2,
		BaseBackoff:             time.Millisecond,
		MaxBackoff:              2 * time.Millisecond,
		CircuitFailureThreshold: 3,
		CircuitOpenDuration:     100 * time.Millisecond,
	}, adapter)

	_, _, err := r.Route(context.Background(), Request{Model: "m"})
	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if got := adapter.Calls(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestRouterCircuitOpensOnRepeatedTransientFailure(t *testing.T) {
	pool := NewCredentialPool()
	pool.Set("primary", []Credential{{ID: "cred-1", Secret: "s"}})
	adapter := &sequenceAdapter{
		provider: "primary",
		results:  []error{&ProviderError{Class: ErrTimeout, Msg: "timeout"}},
	}
	r := NewRouterWithOptions("primary", nil, pool, ResilienceOptions{
		ProviderTimeout:         500 * time.Millisecond,
		MaxAttempts:             1,
		BaseBackoff:             time.Millisecond,
		MaxBackoff:              2 * time.Millisecond,
		CircuitFailureThreshold: 1,
		CircuitOpenDuration:     500 * time.Millisecond,
	}, adapter)

	_, _, err := r.Route(context.Background(), Request{Model: "m"})
	if err == nil {
		t.Fatal("expected first request to fail")
	}
	if got := adapter.Calls(); got != 1 {
		t.Fatalf("expected first call count=1, got %d", got)
	}

	_, _, err = r.Route(context.Background(), Request{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "circuit open") {
		t.Fatalf("expected circuit open error, got: %v", err)
	}
	if got := adapter.Calls(); got != 1 {
		t.Fatalf("expected no additional provider call while circuit open, got %d", got)
	}
}

func TestRouterProviderTimeoutBudget(t *testing.T) {
	pool := NewCredentialPool()
	pool.Set("primary", []Credential{{ID: "cred-1", Secret: "s"}})
	adapter := &sequenceAdapter{
		provider: "primary",
		sleep:    50 * time.Millisecond,
		results:  []error{nil},
	}
	r := NewRouterWithOptions("primary", nil, pool, ResilienceOptions{
		ProviderTimeout:         5 * time.Millisecond,
		MaxAttempts:             1,
		BaseBackoff:             time.Millisecond,
		MaxBackoff:              2 * time.Millisecond,
		CircuitFailureThreshold: 3,
		CircuitOpenDuration:     100 * time.Millisecond,
	}, adapter)

	_, tele, err := r.Route(context.Background(), Request{Model: "m"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}
	if tele.ErrorClass != ErrTimeout {
		t.Fatalf("expected timeout class, got %s", tele.ErrorClass)
	}
}
