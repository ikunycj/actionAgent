package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"actionagent/agent/kernel/events"
	"actionagent/agent/kernel/task"
)

type Executor interface {
	Ready() bool
	Run(context.Context, task.ExecutionEnvelope) (task.Outcome, error)
	Metrics() map[string]any
	SubscribeEvents(buffer int) (<-chan events.Event, func())
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
	s.mux.HandleFunc("/v1/run", s.handleRun)
	s.mux.HandleFunc("/ws/frame", s.handleWSFrame)
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
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
	env := s.normalizeEnvelope("chat.completions", req.SessionKey, req.IdempotencyKey, map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	})
	out, err := s.exec.Run(r.Context(), env)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      out.TaskID,
		"run_id":  out.RunID,
		"state":   out.State,
		"replay":  out.Replay,
		"payload": out.Payload,
	})
}

type runReq struct {
	Input          map[string]any `json:"input"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
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
	env := s.normalizeEnvelope("run", req.SessionKey, req.IdempotencyKey, req.Input)
	out, err := s.exec.Run(r.Context(), env)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
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
		env := s.normalizeEnvelope("run", req.SessionID, "", req.Params)
		out, err := s.exec.Run(r.Context(), env)
		if err != nil {
			res.OK = false
			res.Error = err.Error()
			writeJSON(w, http.StatusOK, res)
			return
		}
		res.Payload = map[string]any{"task_id": out.TaskID, "run_id": out.RunID, "state": out.State}
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

func (s *Server) normalizeEnvelope(operation, sessionKey, idempotencyKey string, input map[string]any) task.ExecutionEnvelope {
	n := atomic.AddUint64(&s.counter, 1)
	now := time.Now().UTC()
	id := fmt.Sprintf("%d-%d", now.UnixNano(), n)
	lane := "main"
	if sessionKey != "" {
		lane = "session:" + sessionKey
	}
	return task.ExecutionEnvelope{
		RequestID:      "req-" + id,
		TaskID:         "task-" + id,
		RunID:          "run-" + id,
		IdempotencyKey: idempotencyKey,
		Lane:           lane,
		SessionKey:     sessionKey,
		Operation:      operation,
		Input:          input,
		CreatedAt:      now,
		Attempt:        1,
		TimeoutMillis:  30000,
	}
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
