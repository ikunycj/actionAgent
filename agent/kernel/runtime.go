package kernel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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
	probe     ReadyProbe

	logger     observability.Logger
	metrics    *observability.Metrics
	events     *events.Bus
	store      storage.KV
	dispatch   *dispatch.Dispatcher
	aggregator *dispatch.TerminalAggregator
	tasks      *task.Engine
	models     *model.Router
	tools      *tools.Runtime
	sessions   *session.TranscriptStore
	memory     memory.Engine
	gateway    *gateway.Server
	httpServer *http.Server

	initOrder []string
}

func NewRuntime(opts StartOptions) *Runtime {
	if opts.AppName == "" {
		opts.AppName = "ActionAgent"
	}
	if opts.EnvVarName == "" {
		opts.EnvVarName = "ACTIONAGENT_CONFIG"
	}
	return &Runtime{opts: opts, metrics: observability.NewMetrics()}
}

func (r *Runtime) Ready() bool { return r.probe.IsReady() }

func (r *Runtime) Metrics() map[string]any {
	if r.metrics == nil {
		return map[string]any{}
	}
	return r.metrics.Snapshot()
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

	if err := r.maybeFail("gateway"); err != nil {
		return err
	}
	r.initSubsystems(ctx)
	r.gateway = gateway.NewServer(r)
	r.httpServer = &http.Server{Addr: r.cfg.HTTPAddr, Handler: r.gateway.Handler()}
	r.initOrder = append(r.initOrder, "gateway")

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
		{ID: "local", Local: true, Healthy: true, Capabilities: map[string]bool{"run": true, "chat.completions": true}},
		{ID: "remote-a", Local: false, Healthy: true, Capabilities: map[string]bool{"run": true}},
	}
	r.dispatch = dispatch.New(nodes)
	r.aggregator = dispatch.NewTerminalAggregator()
	r.tasks = task.NewEngine(task.NewLaneQueue(r.cfg.QueueConcurrency), task.NewDedupeStore(), r.dispatch, r, time.Duration(r.cfg.DedupeTTLSeconds)*time.Second)
	pool := model.NewCredentialPool()
	pool.Set("primary", []model.Credential{{ID: "cred-primary", Secret: "x"}})
	pool.Set("fallback", []model.Credential{{ID: "cred-fallback", Secret: "y"}})
	r.models = model.NewRouter("primary", []string{"fallback"}, pool, model.StaticAdapter{Provider: "primary"}, model.StaticAdapter{Provider: "fallback"})
	reg := tools.NewRegistry()
	reg.Register(tools.Tool{Name: "echo", Tier: tools.L0, Run: func(in map[string]any) (map[string]any, error) {
		return map[string]any{"echo": in}, nil
	}})
	r.tools = tools.NewRuntime(reg, tools.NewApprovalManager())
	r.sessions = session.NewTranscriptStore()
	r.memory = memory.Engine{Vector: nil, FTS: memory.StaticRetriever{Results: []memory.Result{}}}
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
	_ = r.events.Publish(ctx, events.Event{Domain: "lifecycle", Type: "request.accepted", RunID: env.RunID, TaskID: env.TaskID, RequestID: env.RequestID, Payload: map[string]any{"operation": env.Operation}})
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
	_ = r.events.Publish(ctx, events.Event{Domain: "lifecycle", Type: "request.finished", RunID: final.RunID, TaskID: final.TaskID, RequestID: env.RequestID, Payload: map[string]any{"state": final.State, "node": final.NodeID}})
	return final, nil
}

func (r *Runtime) Execute(_ context.Context, env task.ExecutionEnvelope) (map[string]any, error) {
	res, _, err := r.models.Route(context.Background(), model.Request{Provider: "primary", Model: "default", SessionID: env.SessionKey, Input: env.Input})
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"provider": res.Provider,
		"model":    res.Model,
		"output":   res.Output,
	}
	if env.Operation == "run" {
		r.sessions.Append(env.SessionKey, session.TranscriptEntry{Event: "run", Payload: payload})
	}
	return payload, nil
}

func (r *Runtime) UpdateConfig(newCfg config.Settings) (config.ReloadPlan, error) {
	if err := config.Validate(newCfg); err != nil {
		return config.ReloadNoop, err
	}
	plan := config.ClassifyReload(r.cfg, newCfg)
	if err := config.AtomicSave(r.cfgPath, newCfg); err != nil {
		return config.ReloadNoop, err
	}
	r.cfg = newCfg
	_ = r.events.Publish(context.Background(), events.Event{Domain: "system", Type: "config.updated", RunID: "config", Payload: map[string]any{"plan": plan}})
	return plan, nil
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
