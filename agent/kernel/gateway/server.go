package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"actionagent/agent/kernel/events"
	"actionagent/agent/kernel/observability"
	"actionagent/agent/kernel/session"
	"actionagent/agent/kernel/task"
	"actionagent/agent/kernel/tools"
)

type Executor interface {
	Ready() bool
	ResolveAgentID(bodyAgentID, headerAgentID string) (string, error)
	StreamResponses(ctx context.Context, agentID, model string, input any) (*StreamResult, error)
	Run(context.Context, task.ExecutionEnvelope) (task.Outcome, error)
	GetTask(taskID string) (task.Outcome, bool)
	ListTasks(limit int) []task.Outcome
	WaitTask(ctx context.Context, taskID string, timeout time.Duration) (task.Outcome, error)
	QueryAudit(limit int, toolName, decision string) []tools.AuditRecord
	ListApprovalTokens(limit int) []tools.Token
	SessionStats(sessionKey string) session.StoreStats
	MaintainSession(sessionKey string) session.MaintenanceResult
	Alerts() []observability.Alert
	Metrics() map[string]any
	SubscribeEvents(buffer int) (<-chan events.Event, func())
}

type StreamResult struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

type Server struct {
	exec    Executor
	counter uint64
	mux     *http.ServeMux
}

func NewServer(exec Executor) *Server {
	s := &Server{exec: exec, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	s.mux.HandleFunc("/v1/responses", s.handleResponses)
	s.mux.HandleFunc("/v1/run", s.handleRun)
	s.mux.HandleFunc("/ws/frame", s.handleWSFrame)
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/alerts", s.handleAlerts)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	ready := s.exec != nil && s.exec.Ready()
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"ok": ready, "ready": ready, "ts": time.Now().UTC()})
}

type chatReq struct {
	Model          string `json:"model"`
	Messages       []any  `json:"messages"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	SessionKey     string `json:"session_key,omitempty"`
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if len(req.Messages) == 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "messages is required")
		return
	}
	agentID, err := s.resolveAgentID(r, req.AgentID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	env := s.normalizeEnvelope("chat.completions", agentID, req.SessionKey, req.IdempotencyKey, map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	})
	out, err := s.exec.Run(r.Context(), env)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}
	text := extractText(out.Payload)
	modelName := req.Model
	if v, ok := out.Payload["model"].(string); ok && strings.TrimSpace(v) != "" {
		modelName = v
	}
	if text != "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      out.TaskID,
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   modelName,
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": text,
					},
				},
			},
			"run_id":   out.RunID,
			"agent_id": agentID,
			"state":    out.State,
			"replay":   out.Replay,
			"payload":  out.Payload,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       out.TaskID,
		"run_id":   out.RunID,
		"agent_id": agentID,
		"state":    out.State,
		"replay":   out.Replay,
		"payload":  out.Payload,
	})
}

type runReq struct {
	Input          map[string]any `json:"input"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	SessionKey     string         `json:"session_key,omitempty"`
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var req runReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.Input == nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "input is required")
		return
	}
	agentID, err := s.resolveAgentID(r, req.AgentID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	env := s.normalizeEnvelope("run", agentID, req.SessionKey, req.IdempotencyKey, req.Input)
	out, err := s.exec.Run(r.Context(), env)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type responsesReq struct {
	Model          string `json:"model"`
	Input          any    `json:"input"`
	Stream         bool   `json:"stream,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	SessionKey     string `json:"session_key,omitempty"`
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var req responsesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.Input == nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "input is required")
		return
	}
	agentID, err := s.resolveAgentID(r, req.AgentID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if req.Stream {
		s.handleResponsesStream(w, r, req, agentID)
		return
	}
	env := s.normalizeEnvelope("responses.create", agentID, req.SessionKey, req.IdempotencyKey, map[string]any{
		"model": req.Model,
		"input": req.Input,
	})
	out, err := s.exec.Run(r.Context(), env)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}
	status := "failed"
	if out.State == task.StateSucceeded {
		status = "completed"
	}
	text := extractText(out.Payload)
	modelName := req.Model
	if v, ok := out.Payload["model"].(string); ok && strings.TrimSpace(v) != "" {
		modelName = v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          out.TaskID,
		"object":      "response",
		"status":      status,
		"model":       modelName,
		"agent_id":    agentID,
		"output_text": text,
		"run_id":      out.RunID,
		"state":       out.State,
		"replay":      out.Replay,
		"payload":     out.Payload,
	})
}

func (s *Server) handleResponsesStream(w http.ResponseWriter, r *http.Request, req responsesReq, agentID string) {
	if s.exec == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_ready", "executor unavailable")
		return
	}
	stream, err := s.exec.StreamResponses(r.Context(), agentID, req.Model, req.Input)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}
	if stream == nil || stream.Body == nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", "empty stream response")
		return
	}
	defer stream.Body.Close()

	if stream.Header != nil {
		if ct := stream.Header.Get("Content-Type"); strings.TrimSpace(ct) != "" {
			w.Header().Set("Content-Type", ct)
		}
		if cc := stream.Header.Get("Cache-Control"); strings.TrimSpace(cc) != "" {
			w.Header().Set("Cache-Control", cc)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Connection", "keep-alive")

	status := stream.StatusCode
	if status <= 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	flusher, _ := w.(http.Flusher)

	buf := make([]byte, 16*1024)
	for {
		n, err := stream.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
}

// WSFrame defines the typed req/res/event schema used by the bridge endpoint.
type WSFrame struct {
	Type         string         `json:"type"`
	ID           string         `json:"id,omitempty"`
	Method       string         `json:"method,omitempty"`
	Params       map[string]any `json:"params,omitempty"`
	OK           bool           `json:"ok,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
	Error        string         `json:"error,omitempty"`
	Event        string         `json:"event,omitempty"`
	ConnectionID string         `json:"connection_id,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	Seq          uint64         `json:"seq,omitempty"`
}

func (s *Server) handleWSFrame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var req WSFrame
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.Type != "req" {
		writeErr(w, http.StatusBadRequest, "validation_error", "type must be req")
		return
	}
	if strings.TrimSpace(req.Method) == "" {
		writeErr(w, http.StatusBadRequest, "validation_error", "method is required")
		return
	}
	res := WSFrame{Type: "res", ID: req.ID, OK: true, ConnectionID: req.ConnectionID, SessionID: req.SessionID}
	switch req.Method {
	case "agent.run":
		agentID, err := s.resolveAgentID(r, asString(req.Params["agent_id"]))
		if err != nil {
			res.OK = false
			res.Error = err.Error()
			writeJSON(w, http.StatusOK, res)
			return
		}
		env := s.normalizeEnvelope("run", agentID, req.SessionID, "", req.Params)
		out, err := s.exec.Run(r.Context(), env)
		if err != nil {
			res.OK = false
			res.Error = err.Error()
			writeJSON(w, http.StatusOK, res)
			return
		}
		res.Payload = map[string]any{"task_id": out.TaskID, "run_id": out.RunID, "state": out.State, "agent_id": agentID}
	case "agent.wait":
		taskID := asString(req.Params["task_id"])
		if taskID == "" {
			res.OK = false
			res.Error = "task_id is required"
			writeJSON(w, http.StatusOK, res)
			return
		}
		timeout := time.Duration(intFromAny(req.Params["timeout_ms"], 30000)) * time.Millisecond
		out, err := s.exec.WaitTask(r.Context(), taskID, timeout)
		if err != nil {
			res.OK = false
			res.Error = err.Error()
			writeJSON(w, http.StatusOK, res)
			return
		}
		res.Payload = map[string]any{"task": out}
	case "task.get":
		taskID := asString(req.Params["task_id"])
		if taskID == "" {
			res.OK = false
			res.Error = "task_id is required"
			writeJSON(w, http.StatusOK, res)
			return
		}
		out, ok := s.exec.GetTask(taskID)
		if !ok {
			res.OK = false
			res.Error = "task not found"
			writeJSON(w, http.StatusOK, res)
			return
		}
		res.Payload = map[string]any{"task": out}
	case "task.list":
		limit := intFromAny(req.Params["limit"], 20)
		out := s.exec.ListTasks(limit)
		res.Payload = map[string]any{"tasks": out, "count": len(out)}
	case "audit.query":
		limit := intFromAny(req.Params["limit"], 20)
		toolName := asString(req.Params["tool"])
		decision := asString(req.Params["decision"])
		out := s.exec.QueryAudit(limit, toolName, decision)
		res.Payload = map[string]any{"records": out, "count": len(out)}
	case "approval.list":
		limit := intFromAny(req.Params["limit"], 20)
		out := s.exec.ListApprovalTokens(limit)
		res.Payload = map[string]any{"tokens": out, "count": len(out)}
	case "session.stats":
		key := asString(req.Params["session_key"])
		if key == "" {
			key = req.SessionID
		}
		if key == "" {
			res.OK = false
			res.Error = "session_key is required"
			writeJSON(w, http.StatusOK, res)
			return
		}
		stats := s.exec.SessionStats(key)
		res.Payload = map[string]any{"session_key": key, "stats": stats}
	case "session.maintain":
		key := asString(req.Params["session_key"])
		if key == "" {
			key = req.SessionID
		}
		if key == "" {
			res.OK = false
			res.Error = "session_key is required"
			writeJSON(w, http.StatusOK, res)
			return
		}
		out := s.exec.MaintainSession(key)
		res.Payload = map[string]any{"session_key": key, "result": out}
	case "observability.alerts":
		alerts := s.exec.Alerts()
		res.Payload = map[string]any{"alerts": alerts, "count": len(alerts)}
	default:
		res.OK = false
		res.Error = fmt.Sprintf("unsupported method: %s", req.Method)
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if s.exec == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_ready", "executor unavailable")
		return
	}
	ch, cancel := s.exec.SubscribeEvents(16)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			_ = enc.Encode(WSFrame{
				Type:         "event",
				Event:        ev.Type,
				Payload:      ev.Payload,
				ConnectionID: ev.ConnectionID,
				SessionID:    ev.SessionID,
				Seq:          ev.Seq,
			})
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if s.exec == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_ready", "executor unavailable")
		return
	}
	writeJSON(w, http.StatusOK, s.exec.Metrics())
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if s.exec == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_ready", "executor unavailable")
		return
	}
	alerts := s.exec.Alerts()
	writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (s *Server) normalizeEnvelope(operation, agentID, sessionKey, idempotencyKey string, input map[string]any) task.ExecutionEnvelope {
	n := atomic.AddUint64(&s.counter, 1)
	now := time.Now().UTC()
	id := fmt.Sprintf("%d-%d", now.UnixNano(), n)
	lane := "main"
	if strings.TrimSpace(agentID) != "" {
		lane = "agent:" + strings.TrimSpace(agentID) + ":main"
	}
	if sessionKey != "" {
		if strings.TrimSpace(agentID) != "" {
			lane = "agent:" + strings.TrimSpace(agentID) + ":session:" + sessionKey
		} else {
			lane = "session:" + sessionKey
		}
	}
	return task.ExecutionEnvelope{
		RequestID:      "req-" + id,
		TaskID:         "task-" + id,
		RunID:          "run-" + id,
		IdempotencyKey: idempotencyKey,
		AgentID:        strings.TrimSpace(agentID),
		Lane:           lane,
		SessionKey:     sessionKey,
		Operation:      operation,
		Input:          input,
		CreatedAt:      now,
		Attempt:        1,
		TimeoutMillis:  30000,
	}
}

func (s *Server) resolveAgentID(r *http.Request, bodyAgentID string) (string, error) {
	if s.exec == nil {
		return "", errors.New("executor unavailable")
	}
	return s.exec.ResolveAgentID(bodyAgentID, r.Header.Get("X-Agent-ID"))
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func intFromAny(v any, fallback int) int {
	switch x := v.(type) {
	case nil:
		return fallback
	case float64:
		n := int(x)
		if n > 0 {
			return n
		}
	case int:
		if x > 0 {
			return x
		}
	case int64:
		n := int(x)
		if n > 0 {
			return n
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func extractText(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["text"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	if out, ok := payload["output"].(map[string]any); ok {
		if v, ok := out["text"].(string); ok {
			return v
		}
	}
	return ""
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]any{
		"error": map[string]any{
			"code":    errCode,
			"message": msg,
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

var ErrExecutorUnavailable = errors.New("executor unavailable")
