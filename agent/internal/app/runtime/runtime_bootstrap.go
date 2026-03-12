package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"
	"time"

	"actionagent/agent/internal/adapter/httpapi"
	"actionagent/agent/internal/core/dispatch"
	"actionagent/agent/internal/core/memory"
	"actionagent/agent/internal/core/session"
	"actionagent/agent/internal/core/task"
	"actionagent/agent/internal/core/tools"
	"actionagent/agent/internal/platform/config"
	"actionagent/agent/internal/platform/events"
	"actionagent/agent/internal/platform/observability"
	"actionagent/agent/internal/platform/storage"
)

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
	webUIAssetsDir := r.webUIAssetsDir()
	r.gateway = httpapi.NewServer(httpapi.Services{
		Health:  r,
		Agents:  r,
		Catalog: r,
		Streams: r,
		Runner:  r,
		Tasks:   r,
		Audit:   r,
		Session: r,
		Observe: r,
		Events:  r,
	}, webUIAssetsDir)
	bindAddr := r.listenAddr()
	r.httpServer = &http.Server{Addr: bindAddr, Handler: r.gateway.Handler()}
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
			"bind_addr":   bindAddr,
			"webui_dir":   webUIAssetsDir,
			"init_order":  r.initOrder,
		},
	})
	if r.logger != nil {
		r.logger.Info("startup complete", map[string]any{"config_path": r.cfgPath, "bind_addr": bindAddr, "webui_dir": webUIAssetsDir})
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
		GOOS:        goruntime.GOOS,
		EnsureExist: false,
	})
	if err != nil {
		return err
	}
	r.cfgPath = resolved.Path
	r.cfgSource = resolved.Source
	defaults := config.DefaultSettings()
	if r.opts.HTTPAddr != "" {
		if port, err := config.ParseListenPort(r.opts.HTTPAddr); err == nil {
			defaults.Port = port
		}
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

func (r *Runtime) listenAddr() string {
	if strings.TrimSpace(r.opts.HTTPAddr) != "" {
		return strings.TrimSpace(r.opts.HTTPAddr)
	}
	return config.ListenAddr(r.cfg.Port)
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
