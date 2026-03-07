package kernel

import (
	"context"
	"sync/atomic"

	"actionagent/agent/kernel/events"
	"actionagent/agent/kernel/task"
)

// Boundary interfaces keep cross-layer coupling explicit.
type Gateway interface {
	Handler() any
}

type TaskEngine interface {
	Submit(context.Context, task.ExecutionEnvelope) (task.Outcome, error)
}

type Dispatcher interface {
	Choose(context.Context, task.ExecutionEnvelope) (task.NodeChoice, error)
}

type ModelGateway interface {
	Route(context.Context, any) (any, any, error)
}

type ToolRuntime interface {
	Execute(toolName string, input map[string]any, binding any, tokenID string) (map[string]any, error)
}

type SessionMemory interface {
	Query(string, int) ([]any, error)
}

type EventBus interface {
	Publish(context.Context, events.Event) error
}

type ConfigService interface {
	Load() (any, error)
}

type Storage interface {
	Get(string) (string, bool)
	Set(string, string)
}

type Observability interface {
	Snapshot() map[string]any
}

type ReadyProbe struct{ ready atomic.Bool }

func (r *ReadyProbe) Set(v bool)    { r.ready.Store(v) }
func (r *ReadyProbe) IsReady() bool { return r.ready.Load() }
