package runtime

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	coremodel "actionagent/agent/internal/core/model"
	"actionagent/agent/internal/platform/config"
)

type ModelRuntime struct {
	router *coremodel.Router
}

func NewModelRuntime(cfg config.Settings) *ModelRuntime {
	pool := coremodel.NewCredentialPool()
	adapters := []coremodel.Adapter{}
	active := map[string]struct{}{}
	byName := map[string]config.ProviderSettings{}
	for _, p := range cfg.ModelGateway.Providers {
		if !p.Enabled {
			continue
		}
		byName[p.Name] = p
		secret := strings.TrimSpace(p.APIKey)
		if secret == "" && strings.TrimSpace(p.APIKeyEnv) != "" {
			secret = strings.TrimSpace(os.Getenv(p.APIKeyEnv))
		}
		if secret == "" {
			continue
		}
		pool.Set(p.Name, []coremodel.Credential{{ID: p.Name + "-cred-1", Secret: secret}})
		client := &http.Client{Timeout: time.Duration(p.TimeoutMillis) * time.Millisecond}
		adapters = append(adapters, coremodel.NewHTTPAdapter(coremodel.HTTPAdapterConfig{
			Provider:   p.Name,
			APIStyle:   p.APIStyle,
			BaseURL:    p.BaseURL,
			Model:      p.DefaultModel,
			MaxTokens:  p.MaxTokens,
			HTTPClient: client,
		}))
		active[p.Name] = struct{}{}
	}

	if len(adapters) == 0 {
		pool.Set("primary", []coremodel.Credential{{ID: "cred-primary", Secret: "x"}})
		pool.Set("fallback", []coremodel.Credential{{ID: "cred-fallback", Secret: "y"}})
		return &ModelRuntime{
			router: coremodel.NewRouter("primary", []string{"fallback"}, pool, coremodel.StaticAdapter{Provider: "primary"}, coremodel.StaticAdapter{Provider: "fallback"}),
		}
	}

	primary := strings.TrimSpace(cfg.ModelGateway.Primary)
	if _, ok := active[primary]; !ok {
		primary = adapters[0].Name()
	}
	fallbacks := make([]string, 0, len(cfg.ModelGateway.Fallbacks))
	for _, f := range cfg.ModelGateway.Fallbacks {
		f = strings.TrimSpace(f)
		if f == "" || f == primary {
			continue
		}
		if _, ok := active[f]; ok {
			fallbacks = append(fallbacks, f)
		}
	}

	opts := coremodel.DefaultResilienceOptions()
	if p, ok := byName[primary]; ok {
		if p.TimeoutMillis > 0 {
			opts.ProviderTimeout = time.Duration(p.TimeoutMillis) * time.Millisecond
		}
		if p.MaxAttempts > 0 {
			opts.MaxAttempts = p.MaxAttempts
		}
	}
	return &ModelRuntime{
		router: coremodel.NewRouterWithOptions(primary, fallbacks, pool, opts, adapters...),
	}
}

func (m *ModelRuntime) Route(ctx context.Context, req coremodel.Request) (coremodel.Response, coremodel.Telemetry, error) {
	if m == nil || m.router == nil {
		return coremodel.Response{}, coremodel.Telemetry{ErrorClass: coremodel.ErrUnknown}, errors.New("model runtime unavailable")
	}
	return m.router.Route(ctx, req)
}
