package model

import (
	"context"
	"errors"
	"fmt"
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

type Router struct {
	adapters  map[string]Adapter
	primary   string
	fallbacks []string
	creds     *CredentialPool
}

func NewRouter(primary string, fallbacks []string, pool *CredentialPool, adapters ...Adapter) *Router {
	m := map[string]Adapter{}
	for _, a := range adapters {
		m[a.Name()] = a
	}
	if pool == nil {
		pool = NewCredentialPool()
	}
	return &Router{adapters: m, primary: primary, fallbacks: fallbacks, creds: pool}
}

func (r *Router) Route(ctx context.Context, req Request) (Response, Telemetry, error) {
	order := append([]string{r.primary}, r.fallbacks...)
	var lastErr error
	for i, p := range order {
		a := r.adapters[p]
		if a == nil {
			lastErr = fmt.Errorf("adapter not found: %s", p)
			continue
		}
		cred, err := r.creds.pick(p, req.SessionID, time.Now())
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := a.Complete(ctx, req, cred)
		if err == nil {
			return resp, Telemetry{SelectedProvider: p, SelectedModel: req.Model, FallbackStep: i, CredentialID: cred.ID}, nil
		}
		class := classify(err)
		r.creds.ApplyFailure(p, cred.ID, class, time.Now())
		lastErr = err
		if class == ErrAuthPermanent || class == ErrModelNotFound || class == ErrFormat {
			return Response{}, Telemetry{SelectedProvider: p, SelectedModel: req.Model, FallbackStep: i, ErrorClass: class, CredentialID: cred.ID}, err
		}
	}
	return Response{}, Telemetry{ErrorClass: classify(lastErr)}, lastErr
}

func classify(err error) ErrorClass {
	if err == nil {
		return ""
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
