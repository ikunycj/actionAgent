package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"actionagent/agent/internal/adapter/httpapi"
	"actionagent/agent/internal/core/model"
	"actionagent/agent/internal/core/session"
	"actionagent/agent/internal/core/task"
	"actionagent/agent/internal/core/tools"
	"actionagent/agent/internal/platform/events"
)

func (r *Runtime) Run(ctx context.Context, env task.ExecutionEnvelope) (task.Outcome, error) {
	if !r.Ready() || r.tasks == nil {
		return task.Outcome{}, httpapi.ErrExecutorUnavailable
	}
	if strings.TrimSpace(env.AgentID) == "" && r.agents != nil {
		env.AgentID = r.agents.DefaultAgent()
	}
	r.metrics.SetQueueDepth(int64(r.tasks.Pending()))
	if r.dispatch != nil {
		_ = r.dispatch.MarkStale(time.Now())
		r.metrics.SetNodeOnline(int64(r.dispatch.OnlineCount(time.Now())))
	}
	if !env.CreatedAt.IsZero() {
		waitMs := time.Since(env.CreatedAt).Milliseconds()
		if waitMs > 0 {
			r.metrics.AddQueueWait(uint64(waitMs))
		}
	}
	r.metrics.SetActive(atomic.AddInt64(&r.active, 1))
	defer func() {
		r.metrics.SetActive(atomic.AddInt64(&r.active, -1))
		if r.tasks != nil {
			r.metrics.SetQueueDepth(int64(r.tasks.Pending()))
		}
	}()
	_ = r.events.Publish(ctx, events.Event{Domain: "lifecycle", Type: "request.accepted", RunID: env.RunID, TaskID: env.TaskID, RequestID: env.RequestID, Payload: map[string]any{"operation": env.Operation, "agent_id": env.AgentID}})
	out, err := r.tasks.Submit(ctx, env)
	if err != nil {
		_ = r.events.Publish(ctx, events.Event{Domain: "error", Type: "request.failed", RunID: env.RunID, TaskID: env.TaskID, RequestID: env.RequestID, Payload: map[string]any{"error": err.Error()}})
		return task.Outcome{}, err
	}

	if out.State == task.StateFailed && env.Attempt < 2 {
		env.Attempt++
		out2, err2 := r.tasks.Submit(ctx, env)
		if err2 == nil {
			out = out2
		}
	}

	if out.State == task.StateSucceeded {
		r.metrics.IncTaskSuccess()
	} else {
		r.metrics.IncTaskFail()
	}
	final := r.aggregator.Record(out)
	_ = r.events.Publish(ctx, events.Event{Domain: "lifecycle", Type: "request.finished", RunID: final.RunID, TaskID: final.TaskID, RequestID: env.RequestID, Payload: map[string]any{"state": final.State, "node": final.NodeID, "agent_id": env.AgentID}})
	return final, nil
}

func (r *Runtime) Execute(ctx context.Context, env task.ExecutionEnvelope) (map[string]any, error) {
	if r.models == nil || r.agents == nil {
		return nil, errors.New("runtime unavailable")
	}
	agentID := strings.TrimSpace(env.AgentID)
	if agentID == "" {
		agentID = r.agents.DefaultAgent()
	}
	agentRt, ok := r.agents.Get(agentID)
	if !ok {
		return nil, fmt.Errorf("unknown agent_id: %s", agentID)
	}
	scopedSession := agentRt.ScopeSessionKey(env.SessionKey)
	if ms := intFromInput(env.Input["sleep_ms"], 0); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	modelName, _ := env.Input["model"].(string)
	res, tele, err := r.models.Route(ctx, model.Request{
		Provider:  agentRt.ModelProfile(),
		Model:     modelName,
		SessionID: scopedSession,
		Input:     env.Input,
	})
	if err != nil {
		r.metrics.Inc("model_route_fail")
		r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_fail")
		if tele.ErrorClass != "" {
			r.metrics.Inc("model_error_" + sanitizeMetricLabel(string(tele.ErrorClass)))
			r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_error_" + sanitizeMetricLabel(string(tele.ErrorClass)))
		}
		return nil, err
	}
	r.metrics.Inc("model_route_ok")
	r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_route_ok")
	r.metrics.Inc("model_provider_" + sanitizeMetricLabel(tele.SelectedProvider))
	r.metrics.Inc("model_agent_" + sanitizeMetricLabel(agentID) + "_provider_" + sanitizeMetricLabel(tele.SelectedProvider))
	r.metrics.SetGauge("model_fallback_step", int64(tele.FallbackStep))
	payload := map[string]any{
		"agent_id":      agentID,
		"model_profile": agentRt.ModelProfile(),
		"provider":      res.Provider,
		"model":         res.Model,
		"output":        res.Output,
		"fallback_step": tele.FallbackStep,
		"credential_id": tele.CredentialID,
	}
	if text, ok := res.Output["text"].(string); ok {
		payload["text"] = text
	}
	if env.Operation == "run" {
		r.sessions.Append(scopedSession, session.TranscriptEntry{Event: "run", Payload: payload})
	}
	return payload, nil
}

func (r *Runtime) GetTask(taskID, agentID string) (task.Outcome, bool) {
	if r.aggregator == nil {
		return task.Outcome{}, false
	}
	out, ok := r.aggregator.Get(taskID)
	if !ok {
		return task.Outcome{}, false
	}
	if !matchesAgentScope(out, agentID) {
		return task.Outcome{}, false
	}
	return out, true
}

func (r *Runtime) ListTasks(agentID string, limit int) []task.Outcome {
	if r.aggregator == nil {
		return []task.Outcome{}
	}
	items := r.aggregator.List(0)
	if strings.TrimSpace(agentID) == "" {
		if limit > 0 && len(items) > limit {
			return items[:limit]
		}
		return items
	}
	filtered := make([]task.Outcome, 0, len(items))
	for _, out := range items {
		if matchesAgentScope(out, agentID) {
			filtered = append(filtered, out)
		}
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered
}

func (r *Runtime) WaitTask(ctx context.Context, taskID, agentID string, timeout time.Duration) (task.Outcome, error) {
	if strings.TrimSpace(taskID) == "" {
		return task.Outcome{}, errors.New("task_id is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if out, ok := r.GetTask(taskID, agentID); ok {
			return out, nil
		}
		select {
		case <-waitCtx.Done():
			return task.Outcome{}, waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func (r *Runtime) QueryAudit(limit int, toolName, decision string) []tools.AuditRecord {
	if r.tools == nil {
		return []tools.AuditRecord{}
	}
	return r.tools.QueryAudit(limit, toolName, decision)
}

func (r *Runtime) ListApprovalTokens(limit int) []tools.Token {
	if r.tools == nil {
		return []tools.Token{}
	}
	return r.tools.ListApprovalTokens(limit)
}

func (r *Runtime) SessionStats(sessionKey string) session.StoreStats {
	if r.sessions == nil {
		return session.StoreStats{}
	}
	return r.sessions.Stats(sessionKey)
}

func (r *Runtime) MaintainSession(sessionKey string) session.MaintenanceResult {
	if r.sessions == nil {
		return session.MaintenanceResult{}
	}
	return r.sessions.ApplyPolicy(sessionKey, time.Now().UTC())
}

func (r *Runtime) DefaultAgentID() string {
	if r.agents == nil {
		return ""
	}
	return r.agents.DefaultAgent()
}

func (r *Runtime) ListAgentIDs() []string {
	if r.agents == nil {
		return nil
	}
	return r.agents.IDs()
}

func matchesAgentScope(out task.Outcome, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return true
	}
	if strings.TrimSpace(out.AgentID) != "" {
		return strings.TrimSpace(out.AgentID) == agentID
	}
	if out.Payload == nil {
		return false
	}
	if payloadAgentID, _ := out.Payload["agent_id"].(string); strings.TrimSpace(payloadAgentID) != "" {
		return strings.TrimSpace(payloadAgentID) == agentID
	}
	return false
}

func intFromInput(v any, fallback int) int {
	switch x := v.(type) {
	case nil:
		return fallback
	case int:
		if x > 0 {
			return x
		}
	case int64:
		n := int(x)
		if n > 0 {
			return n
		}
	case float64:
		n := int(x)
		if n > 0 {
			return n
		}
	}
	return fallback
}

func sanitizeMetricLabel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "unknown"
	}
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}
