package kernel

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"actionagent/agent/kernel/config"
)

type agentRegistrySnapshot struct {
	defaultAgent string
	agents       map[string]*AgentRuntime
}

type AgentRegistry struct {
	snapshot atomic.Value // *agentRegistrySnapshot
}

func NewAgentRegistry(cfg config.Settings) (*AgentRegistry, error) {
	s, err := buildAgentRegistrySnapshot(cfg)
	if err != nil {
		return nil, err
	}
	r := &AgentRegistry{}
	r.snapshot.Store(s)
	return r, nil
}

func buildAgentRegistrySnapshot(cfg config.Settings) (*agentRegistrySnapshot, error) {
	agents := map[string]*AgentRuntime{}
	for _, a := range cfg.Agents {
		if !a.Enabled {
			continue
		}
		id := strings.TrimSpace(a.AgentID)
		if id == "" {
			continue
		}
		agents[id] = NewAgentRuntime(a)
	}
	def := strings.TrimSpace(cfg.DefaultAgent)
	if def == "" {
		return nil, fmt.Errorf("default_agent is required")
	}
	if _, ok := agents[def]; !ok {
		return nil, fmt.Errorf("default_agent %q not found", def)
	}
	return &agentRegistrySnapshot{
		defaultAgent: def,
		agents:       agents,
	}, nil
}

func (r *AgentRegistry) Swap(cfg config.Settings) error {
	s, err := buildAgentRegistrySnapshot(cfg)
	if err != nil {
		return err
	}
	r.snapshot.Store(s)
	return nil
}

func (r *AgentRegistry) Resolve(bodyAgentID, headerAgentID string) (string, error) {
	s := r.current()
	if s == nil {
		return "", fmt.Errorf("agent registry unavailable")
	}
	choice := strings.TrimSpace(bodyAgentID)
	if choice == "" {
		choice = strings.TrimSpace(headerAgentID)
	}
	if choice == "" {
		return s.defaultAgent, nil
	}
	if _, ok := s.agents[choice]; !ok {
		return "", fmt.Errorf("unknown agent_id: %s", choice)
	}
	return choice, nil
}

func (r *AgentRegistry) Get(agentID string) (*AgentRuntime, bool) {
	s := r.current()
	if s == nil {
		return nil, false
	}
	agentID = strings.TrimSpace(agentID)
	a, ok := s.agents[agentID]
	return a, ok
}

func (r *AgentRegistry) DefaultAgent() string {
	s := r.current()
	if s == nil {
		return ""
	}
	return s.defaultAgent
}

func (r *AgentRegistry) IDs() []string {
	s := r.current()
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.agents))
	for id := range s.agents {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (r *AgentRegistry) current() *agentRegistrySnapshot {
	if r == nil {
		return nil
	}
	v := r.snapshot.Load()
	if v == nil {
		return nil
	}
	s, _ := v.(*agentRegistrySnapshot)
	return s
}
