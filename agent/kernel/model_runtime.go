package kernel

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"actionagent/agent/kernel/config"
	"actionagent/agent/kernel/model"
)

type ModelRuntime struct {
	router *model.Router
}

func NewModelRuntime(cfg config.Settings) *ModelRuntime {
	pool := model.NewCredentialPool()
	adapters := []model.Adapter{}
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
		pool.Set(p.Name, []model.Credential{{ID: p.Name + "-cred-1", Secret: secret}})
		client := &http.Client{Timeout: time.Duration(p.TimeoutMillis) * time.Millisecond}
		adapters = append(adapters, model.NewHTTPAdapter(model.HTTPAdapterConfig{
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
		pool.Set("primary", []model.Credential{{ID: "cred-primary", Secret: "x"}})
		pool.Set("fallback", []model.Credential{{ID: "cred-fallback", Secret: "y"}})
		return &ModelRuntime{
			router: model.NewRouter("primary", []string{"fallback"}, pool, model.StaticAdapter{Provider: "primary"}, model.StaticAdapter{Provider: "fallback"}),
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

	opts := model.DefaultResilienceOptions()
	if p, ok := byName[primary]; ok {
		if p.TimeoutMillis > 0 {
			opts.ProviderTimeout = time.Duration(p.TimeoutMillis) * time.Millisecond
		}
		if p.MaxAttempts > 0 {
			opts.MaxAttempts = p.MaxAttempts
		}
	}
	return &ModelRuntime{
		router: model.NewRouterWithOptions(primary, fallbacks, pool, opts, adapters...),
	}
}

func (m *ModelRuntime) Route(ctx context.Context, req model.Request) (model.Response, model.Telemetry, error) {
	if m == nil || m.router == nil {
		return model.Response{}, model.Telemetry{ErrorClass: model.ErrUnknown}, errors.New("model runtime unavailable")
	}
	return m.router.Route(ctx, req)
}
