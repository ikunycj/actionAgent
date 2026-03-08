package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"actionagent/agent/kernel/config"
	"actionagent/agent/kernel/dispatch"
	"actionagent/agent/kernel/events"
	"actionagent/agent/kernel/gateway"
	"actionagent/agent/kernel/memory"
	"actionagent/agent/kernel/model"
	"actionagent/agent/kernel/observability"
	"actionagent/agent/kernel/session"
	"actionagent/agent/kernel/storage"
	"actionagent/agent/kernel/task"
	"actionagent/agent/kernel/tools"
)

type StartOptions struct {
	CLIConfigPath string
	BinaryPath    string
	AppName       string
	HTTPAddr      string
	EnvVarName    string
	InitFailures  map[string]error
}

type Runtime struct {
	opts      StartOptions
	cfgPath   string
	cfg       config.Settings
	cfgSource config.Source
	legacyCfg bool
	probe     ReadyProbe

	logger     observability.Logger
	metrics    *observability.Metrics
	events     *events.Bus
	store      storage.KV
	dispatch   *dispatch.Dispatcher
	aggregator *dispatch.TerminalAggregator
	tasks      *task.Engine
	models     *ModelRuntime
	agents     *AgentRegistry
	tools      *tools.Runtime
	sessions   *session.TranscriptStore
	memory     memory.Engine
	gateway    *gateway.Server
	httpServer *http.Server

	initOrder []string
	active    int64
	alerts    observability.AlertThresholds
}

func NewRuntime(opts StartOptions) *Runtime {
	if opts.AppName == "" {
		opts.AppName = "ActionAgent"
	}
	if opts.EnvVarName == "" {
		opts.EnvVarName = "ACTIONAGENT_CONFIG"
	}
	return &Runtime{
		opts:    opts,
		metrics: observability.NewMetrics(),
		alerts:  observability.DefaultAlertThresholds(),
	}
}

func (r *Runtime) Ready() bool { return r.probe.IsReady() }

func (r *Runtime) Metrics() map[string]any {
	if r.metrics == nil {
		return map[string]any{}
	}
	return r.metrics.Snapshot()
}

func (r *Runtime) Alerts() []observability.Alert {
	if r.metrics == nil {
		return []observability.Alert{}
	}
	return observability.EvaluateAlerts(r.metrics.Snapshot(), r.alerts)
}

func (r *Runtime) SubscribeEvents(buffer int) (<-chan events.Event, func()) {
	if r.events == nil {
		ch := make(chan events.Event)
		close(ch)
		return ch, func() {}
	}
	return r.events.Subscribe(buffer)
}

func (r *Runtime) Init(ctx context.Context) error {
	if err := r.initConfig(); err != nil {
		return fmt.Errorf("config init: %w", err)
	}
	r.initOrder = append(r.initOrder, "config")

	if err := r.maybeFail("logging"); err != nil {
		return err
	}
	r.logger = observability.NewStdLogger()
	r.logger.Info("logging initialized", map[string]any{"level": r.cfg.LogLevel})
	r.initOrder = append(r.initOrder, "logging")

	if err := r.maybeFail("events"); err != nil {
		return err
	}
	r.events = events.NewBus(events.NewInMemorySink())
	r.initOrder = append(r.initOrder, "events")

	if err := r.maybeFail("storage"); err != nil {
		return err
	}
	r.store = storage.NewInMemoryKV()
	r.initOrder = append(r.initOrder, "storage")

	r.initSubsystems(ctx)

	if err := r.maybeFail("model-runtime"); err != nil {
		return err
	}
	r.models = NewModelRuntime(r.cfg)
	r.initOrder = append(r.initOrder, "model-runtime")

	if err := r.maybeFail("agent-registry"); err != nil {
		return err
	}
	agents, err := NewAgentRegistry(r.cfg)
	if err != nil {
		return fmt.Errorf("agent registry init: %w", err)
	}
	r.agents = agents
	r.initOrder = append(r.initOrder, "agent-registry")

	if err := r.maybeFail("gateway"); err != nil {
		return err
	}
	r.gateway = gateway.NewServer(r)
	r.httpServer = &http.Server{Addr: r.cfg.HTTPAddr, Handler: r.gateway.Handler()}
	r.initOrder = append(r.initOrder, "gateway")

	if r.legacyCfg {
		r.metrics.Inc("config_legacy_agent_synthesized")
		if r.logger != nil {
			r.logger.Info("legacy config detected, synthesized default agent", map[string]any{"default_agent": r.cfg.DefaultAgent})
		}
	}

	r.probe.Set(true)
	_ = r.events.Publish(ctx, events.Event{
		Domain: "system",
		Type:   "startup.complete",
		RunID:  "startup",
		Payload: map[string]any{
			"config_path": r.cfgPath,
			"bind_addr":   r.cfg.HTTPAddr,
			"init_order":  r.initOrder,
		},
	})
	if r.logger != nil {
		r.logger.Info("startup complete", map[string]any{"config_path": r.cfgPath, "bind_addr": r.cfg.HTTPAddr})
	}
	return nil
}

func (r *Runtime) initSubsystems(_ context.Context) {
	nodes := []dispatch.Node{
		{ID: "local", Local: true, Healthy: true, Capabilities: map[string]bool{"run": true, "chat.completions": true, "responses.create": true}},
		{ID: "remote-a", Local: false, Healthy: true, Capabilities: map[string]bool{"run": true}},
	}
	r.dispatch = dispatch.New(nodes)
	r.aggregator = dispatch.NewTerminalAggregator()
	r.tasks = task.NewEngine(task.NewLaneQueue(r.cfg.QueueConcurrency), task.NewDedupeStore(), r.dispatch, r, time.Duration(r.cfg.DedupeTTLSeconds)*time.Second)
	reg := tools.NewRegistry()
	reg.Register(tools.Tool{Name: "echo", Tier: tools.L0, Run: func(in map[string]any) (map[string]any, error) {
		return map[string]any{"echo": in}, nil
	}})
	toolStatePath := filepath.Join(filepath.Dir(r.cfgPath), "state", "tools-state.json")
	r.tools = tools.NewRuntimeWithState(reg, tools.NewApprovalManager(), toolStatePath)
	r.sessions = session.NewTranscriptStoreWithPolicy(session.MaintenancePolicy{
		Mode:       session.EnforceMode,
		PruneAfter: 7 * 24 * time.Hour,
		MaxEntries: 500,
		MaxDisk:    2 * 1024 * 1024,
	})
	r.memory = memory.Engine{Vector: nil, FTS: memory.StaticRetriever{Results: []memory.Result{}}}
	r.metrics.SetNodeOnline(int64(r.dispatch.OnlineCount(time.Now())))
}

func (r *Runtime) Start(ctx context.Context) error {
	if err := r.Init(ctx); err != nil {
		return err
	}
	go func() {
		if err := r.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && r.logger != nil {
			r.logger.Error("http server failed", map[string]any{"error": err.Error()})
		}
	}()
	return nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	r.probe.Set(false)
	if r.tasks != nil {
		r.tasks.EnterDraining()
	}
	if r.httpServer != nil {
		return r.httpServer.Shutdown(ctx)
	}
	return nil
}

func (r *Runtime) Run(ctx context.Context, env task.ExecutionEnvelope) (task.Outcome, error) {
	if !r.Ready() || r.tasks == nil {
		return task.Outcome{}, gateway.ErrExecutorUnavailable
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
		// Retry coordination between task engine and dispatch handoff path.
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

func (r *Runtime) GetTask(taskID string) (task.Outcome, bool) {
	if r.aggregator == nil {
		return task.Outcome{}, false
	}
	return r.aggregator.Get(taskID)
}

func (r *Runtime) ListTasks(limit int) []task.Outcome {
	if r.aggregator == nil {
		return []task.Outcome{}
	}
	return r.aggregator.List(limit)
}

func (r *Runtime) WaitTask(ctx context.Context, taskID string, timeout time.Duration) (task.Outcome, error) {
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
		if out, ok := r.GetTask(taskID); ok {
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

func (r *Runtime) StreamResponses(ctx context.Context, agentID, modelName string, input any) (*gateway.StreamResult, error) {
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

	return &gateway.StreamResult{
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

func (r *Runtime) initConfig() error {
	binaryDir := ""
	if r.opts.BinaryPath != "" {
		binaryDir = filepath.Dir(r.opts.BinaryPath)
	}
	resolved, err := config.ResolvePath(config.ResolveInput{
		CLIPath:     r.opts.CLIConfigPath,
		EnvPath:     os.Getenv(r.opts.EnvVarName),
		BinaryDir:   binaryDir,
		AppName:     r.opts.AppName,
		GOOS:        runtime.GOOS,
		EnsureExist: false,
	})
	if err != nil {
		return err
	}
	r.cfgPath = resolved.Path
	r.cfgSource = resolved.Source
	defaults := config.DefaultSettings(r.opts.AppName)
	if r.opts.HTTPAddr != "" {
		defaults.HTTPAddr = r.opts.HTTPAddr
	}
	if err := config.EnsureConfig(r.cfgPath, defaults); err != nil {
		return err
	}
	cfg, err := config.LoadSingleSource(r.cfgPath)
	if err != nil {
		return err
	}
	r.legacyCfg = cfg.LegacyImplicitDefault
	r.cfg = cfg
	return nil
}

func (r *Runtime) maybeFail(name string) error {
	if r.opts.InitFailures == nil {
		return nil
	}
	if err, ok := r.opts.InitFailures[name]; ok {
		return fmt.Errorf("%s init failed: %w", name, err)
	}
	return nil
}

func WaitForSignal(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	cancel()
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
