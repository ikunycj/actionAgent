package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"actionagent/agent/internal/adapter/httpapi"
	"actionagent/agent/internal/platform/config"
	"actionagent/agent/internal/platform/events"
)

func (r *Runtime) UpdateConfig(newCfg config.Settings) (config.ReloadPlan, error) {
	next := newCfg
	config.Normalize(&next)
	if err := config.Validate(next); err != nil {
		return config.ReloadNoop, err
	}
	nextModels := NewModelRuntime(next)
	nextAgents, err := NewAgentRegistry(next)
	if err != nil {
		return config.ReloadNoop, err
	}
	plan := config.ClassifyReload(r.cfg, next)
	if err := config.AtomicSave(r.cfgPath, next); err != nil {
		return config.ReloadNoop, err
	}
	r.cfg = next
	r.legacyCfg = next.LegacyImplicitDefault
	r.models = nextModels
	r.agents = nextAgents
	if r.legacyCfg {
		r.metrics.Inc("config_legacy_agent_synthesized")
	}
	_ = r.events.Publish(context.Background(), events.Event{Domain: "system", Type: "config.updated", RunID: "config", Payload: map[string]any{"plan": plan}})
	return plan, nil
}

func (r *Runtime) ResolveAgentID(bodyAgentID, headerAgentID string) (string, error) {
	if r.agents == nil {
		return "", errors.New("agent registry unavailable")
	}
	return r.agents.Resolve(bodyAgentID, headerAgentID)
}

func (r *Runtime) StreamResponses(ctx context.Context, agentID, modelName string, input any) (*httpapi.StreamResult, error) {
	if r.agents == nil {
		return nil, errors.New("agent registry unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = r.agents.DefaultAgent()
	}
	agentRt, ok := r.agents.Get(agentID)
	if !ok {
		return nil, fmt.Errorf("unknown agent_id: %s", agentID)
	}

	providerName := strings.TrimSpace(agentRt.ModelProfile())
	if providerName == "" {
		providerName = strings.TrimSpace(r.cfg.ModelGateway.Primary)
	}
	provider, err := r.findEnabledProvider(providerName)
	if err != nil {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_unknown")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_unknown")
		return nil, err
	}
	if strings.TrimSpace(strings.ToLower(provider.APIStyle)) != "openai" {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_format")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_format")
		return nil, fmt.Errorf("streaming responses requires openai api_style, got: %s", provider.APIStyle)
	}

	secret := strings.TrimSpace(provider.APIKey)
	if secret == "" && strings.TrimSpace(provider.APIKeyEnv) != "" {
		secret = strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.APIKeyEnv)))
	}
	if secret == "" {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_auth")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_auth")
		return nil, fmt.Errorf("provider %s has no credential", providerName)
	}

	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		modelName = strings.TrimSpace(provider.DefaultModel)
	}
	if modelName == "" {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_format")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_format")
		return nil, errors.New("model is required")
	}

	payload := map[string]any{
		"model":  modelName,
		"input":  input,
		"stream": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_format")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_format")
		return nil, err
	}

	endpoint := strings.TrimRight(provider.BaseURL, "/") + "/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_unknown")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_unknown")
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "ActionAgent/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_unknown")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_unknown")
		return nil, err
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		r.metrics.Inc("model_error_format")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_format")
		return nil, errors.New(extractUpstreamErrorMessage(resp.StatusCode, raw))
	}

	r.metrics.Inc("model_route_ok")
	r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_ok")
	r.metrics.Inc("model_provider_" + sanitizeMetricLabel(providerName))
	r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_provider_" + sanitizeMetricLabel(providerName))

	return &httpapi.StreamResult{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       resp.Body,
	}, nil
}

func (r *Runtime) findEnabledProvider(name string) (config.ProviderSettings, error) {
	name = strings.TrimSpace(name)
	for _, p := range r.cfg.ModelGateway.Providers {
		if strings.TrimSpace(p.Name) == name && p.Enabled {
			return p, nil
		}
	}
	return config.ProviderSettings{}, fmt.Errorf("provider not enabled: %s", name)
}

func extractUpstreamErrorMessage(status int, raw []byte) string {
	msg := strings.TrimSpace(string(raw))
	var out struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err == nil && strings.TrimSpace(out.Error.Message) != "" {
		msg = strings.TrimSpace(out.Error.Message)
	}
	if msg == "" {
		return fmt.Sprintf("Upstream request failed (status=%d)", status)
	}
	return msg
}
