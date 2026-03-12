package runtime

import (
	"net/http"

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

type StartOptions struct {
	CLIConfigPath  string
	BinaryPath     string
	AppName        string
	HTTPAddr       string
	WebUIAssetsDir string
	EnvVarName     string
	InitFailures   map[string]error
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
	gateway    *httpapi.Server
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
