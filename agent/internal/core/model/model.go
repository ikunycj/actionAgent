package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ErrorClass string

const (
	ErrRateLimit     ErrorClass = "rate_limit"
	ErrTimeout       ErrorClass = "timeout"
	ErrAuth          ErrorClass = "auth"
	ErrAuthPermanent ErrorClass = "auth_permanent"
	ErrBilling       ErrorClass = "billing"
	ErrFormat        ErrorClass = "format"
	ErrModelNotFound ErrorClass = "model_not_found"
	ErrUnknown       ErrorClass = "unknown"
)

type ProviderError struct {
	Class ErrorClass
	Msg   string
}

func (e *ProviderError) Error() string { return e.Msg }

type Request struct {
	Provider  string
	Model     string
	SessionID string
	Input     map[string]any
}

type Response struct {
	Provider string
	Model    string
	Output   map[string]any
}

type Adapter interface {
	Name() string
	Complete(context.Context, Request, Credential) (Response, error)
}

type Credential struct {
	ID     string
	Secret string
}

type credState struct {
	cooldownUntil time.Time
	disableUntil  time.Time
}

type CredentialPool struct {
	mu     sync.Mutex
	byProv map[string][]Credential
	states map[string]credState
	sticky map[string]string
}

func NewCredentialPool() *CredentialPool {
	return &CredentialPool{byProv: map[string][]Credential{}, states: map[string]credState{}, sticky: map[string]string{}}
}

func (p *CredentialPool) Set(provider string, creds []Credential) {
	p.mu.Lock()
	p.byProv[provider] = creds
	p.mu.Unlock()
}

func (p *CredentialPool) pick(provider, sessionID string, now time.Time) (Credential, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sid := p.sticky[sessionID]; sid != "" {
		for _, c := range p.byProv[provider] {
			if c.ID == sid && p.allowedLocked(provider, c.ID, now) {
				return c, nil
			}
		}
	}
	for _, c := range p.byProv[provider] {
		if p.allowedLocked(provider, c.ID, now) {
			p.sticky[sessionID] = c.ID
			return c, nil
		}
	}
	return Credential{}, errors.New("no eligible credential")
}

func (p *CredentialPool) allowedLocked(provider, id string, now time.Time) bool {
	st := p.states[provider+":"+id]
	if now.Before(st.disableUntil) {
		return false
	}
	if now.Before(st.cooldownUntil) {
		return false
	}
	return true
}

func (p *CredentialPool) ApplyFailure(provider, credID string, class ErrorClass, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := provider + ":" + credID
	st := p.states[key]
	switch class {
	case ErrRateLimit, ErrTimeout:
		st.cooldownUntil = now.Add(30 * time.Second)
	case ErrBilling:
		st.disableUntil = now.Add(24 * time.Hour)
	}
	p.states[key] = st
}

type Telemetry struct {
	SelectedProvider string
	SelectedModel    string
	FallbackStep     int
	ErrorClass       ErrorClass
	CredentialID     string
}

type ResilienceOptions struct {
	ProviderTimeout         time.Duration
	MaxAttempts             int
	BaseBackoff             time.Duration
	MaxBackoff              time.Duration
	CircuitFailureThreshold int
	CircuitOpenDuration     time.Duration
}

func DefaultResilienceOptions() ResilienceOptions {
	return ResilienceOptions{
		ProviderTimeout:         20 * time.Second,
		MaxAttempts:             2,
		BaseBackoff:             100 * time.Millisecond,
		MaxBackoff:              2 * time.Second,
		CircuitFailureThreshold: 3,
		CircuitOpenDuration:     15 * time.Second,
	}
}

func normalizeOptions(opts ResilienceOptions) ResilienceOptions {
	if opts.ProviderTimeout <= 0 {
		opts.ProviderTimeout = 20 * time.Second
	}
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}
	if opts.BaseBackoff <= 0 {
		opts.BaseBackoff = 100 * time.Millisecond
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 2 * time.Second
	}
	if opts.MaxBackoff < opts.BaseBackoff {
		opts.MaxBackoff = opts.BaseBackoff
	}
	if opts.CircuitFailureThreshold < 1 {
		opts.CircuitFailureThreshold = 3
	}
	if opts.CircuitOpenDuration <= 0 {
		opts.CircuitOpenDuration = 15 * time.Second
	}
	return opts
}

type circuitState struct {
	consecutive int
	openUntil   time.Time
}

type providerCircuit struct {
	mu     sync.Mutex
	opts   ResilienceOptions
	states map[string]circuitState
}

func newProviderCircuit(opts ResilienceOptions) *providerCircuit {
	return &providerCircuit{opts: opts, states: map[string]circuitState{}}
}

func (c *providerCircuit) allow(provider string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.states[provider]
	return !now.Before(st.openUntil)
}

func (c *providerCircuit) success(provider string) {
	c.mu.Lock()
	c.states[provider] = circuitState{}
	c.mu.Unlock()
}

func (c *providerCircuit) failure(provider string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.states[provider]
	st.consecutive++
	if st.consecutive >= c.opts.CircuitFailureThreshold {
		st.openUntil = now.Add(c.opts.CircuitOpenDuration)
		st.consecutive = 0
	}
	c.states[provider] = st
}

type Router struct {
	adapters  map[string]Adapter
	primary   string
	fallbacks []string
	creds     *CredentialPool
	opts      ResilienceOptions
	circuit   *providerCircuit
}

func NewRouter(primary string, fallbacks []string, pool *CredentialPool, adapters ...Adapter) *Router {
	return NewRouterWithOptions(primary, fallbacks, pool, DefaultResilienceOptions(), adapters...)
}

func NewRouterWithOptions(primary string, fallbacks []string, pool *CredentialPool, opts ResilienceOptions, adapters ...Adapter) *Router {
	m := map[string]Adapter{}
	for _, a := range adapters {
		m[a.Name()] = a
	}
	if pool == nil {
		pool = NewCredentialPool()
	}
	opts = normalizeOptions(opts)
	return &Router{
		adapters:  m,
		primary:   primary,
		fallbacks: fallbacks,
		creds:     pool,
		opts:      opts,
		circuit:   newProviderCircuit(opts),
	}
}

func (r *Router) Route(ctx context.Context, req Request) (Response, Telemetry, error) {
	order := make([]string, 0, 1+len(r.fallbacks)+1)
	seen := map[string]struct{}{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		order = append(order, p)
	}
	add(req.Provider)
	add(r.primary)
	for _, f := range r.fallbacks {
		add(f)
	}
	var lastErr error
	for i, p := range order {
		a := r.adapters[p]
		if a == nil {
			lastErr = fmt.Errorf("adapter not found: %s", p)
			continue
		}
		now := time.Now()
		if !r.circuit.allow(p, now) {
			lastErr = fmt.Errorf("provider circuit open: %s", p)
			continue
		}
		cred, err := r.creds.pick(p, req.SessionID, time.Now())
		if err != nil {
			lastErr = err
			continue
		}
		resp, class, err := r.callWithResilience(ctx, a, req, cred)
		if err == nil {
			r.circuit.success(p)
			return resp, Telemetry{SelectedProvider: p, SelectedModel: req.Model, FallbackStep: i, CredentialID: cred.ID}, nil
		}
		r.creds.ApplyFailure(p, cred.ID, class, now)
		if shouldOpenCircuit(class) {
			r.circuit.failure(p, now)
		}
		lastErr = err
		if class == ErrAuthPermanent || class == ErrModelNotFound || class == ErrFormat {
			return Response{}, Telemetry{SelectedProvider: p, SelectedModel: req.Model, FallbackStep: i, ErrorClass: class, CredentialID: cred.ID}, err
		}
	}
	return Response{}, Telemetry{ErrorClass: classify(lastErr)}, lastErr
}

func (r *Router) callWithResilience(ctx context.Context, a Adapter, req Request, cred Credential) (Response, ErrorClass, error) {
	var lastErr error
	var class ErrorClass
	for attempt := 1; attempt <= r.opts.MaxAttempts; attempt++ {
		callCtx := ctx
		cancel := func() {}
		if r.opts.ProviderTimeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, r.opts.ProviderTimeout)
		}
		resp, err := a.Complete(callCtx, req, cred)
		cancel()
		if err == nil {
			return resp, "", nil
		}
		class = classify(err)
		lastErr = err
		if !retryable(class) || attempt == r.opts.MaxAttempts {
			break
		}
		backoff := r.backoffFor(attempt)
		select {
		case <-ctx.Done():
			return Response{}, classify(ctx.Err()), ctx.Err()
		case <-time.After(backoff):
		}
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
		class = classify(lastErr)
	}
	return Response{}, class, lastErr
}

func (r *Router) backoffFor(attempt int) time.Duration {
	multiplier := 1 << (attempt - 1)
	wait := time.Duration(multiplier) * r.opts.BaseBackoff
	if wait > r.opts.MaxBackoff {
		return r.opts.MaxBackoff
	}
	return wait
}

func retryable(class ErrorClass) bool {
	switch class {
	case ErrRateLimit, ErrTimeout, ErrUnknown:
		return true
	default:
		return false
	}
}

func shouldOpenCircuit(class ErrorClass) bool {
	return retryable(class)
}

func classify(err error) ErrorClass {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Class
	}
	return ErrUnknown
}

type StaticAdapter struct {
	Provider string
	Fail     error
}

func (a StaticAdapter) Name() string { return a.Provider }
func (a StaticAdapter) Complete(_ context.Context, req Request, _ Credential) (Response, error) {
	if a.Fail != nil {
		return Response{}, a.Fail
	}
	return Response{Provider: a.Provider, Model: req.Model, Output: map[string]any{"ok": true}}, nil
}
