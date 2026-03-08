package kernel

import (
	"fmt"
	"strings"

	"actionagent/agent/kernel/config"
)

type AgentRuntime struct {
	cfg config.AgentSettings
}

func NewAgentRuntime(cfg config.AgentSettings) *AgentRuntime {
	return &AgentRuntime{cfg: cfg}
}

func (a *AgentRuntime) ID() string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(a.cfg.AgentID)
}

func (a *AgentRuntime) ModelProfile() string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(a.cfg.ModelProfile)
}

func (a *AgentRuntime) ScopeSessionKey(sessionKey string) string {
	agentID := a.ID()
	if agentID == "" {
		agentID = "unknown"
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return fmt.Sprintf("agent:%s:main", agentID)
	}
	return fmt.Sprintf("agent:%s:session:%s", agentID, sessionKey)
}
