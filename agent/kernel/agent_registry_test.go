package kernel

import (
	"testing"

	"actionagent/agent/kernel/config"
)

func TestAgentRegistryResolvePrecedence(t *testing.T) {
	cfg := config.DefaultSettings("ActionAgent")
	cfg.Agents = []config.AgentSettings{
		{AgentID: "alpha", Enabled: true, ModelProfile: "openai-main"},
		{AgentID: "beta", Enabled: true, ModelProfile: "openai-main"},
	}
	cfg.DefaultAgent = "alpha"

	reg, err := NewAgentRegistry(cfg)
	if err != nil {
		t.Fatalf("new agent registry failed: %v", err)
	}
	got, err := reg.Resolve("beta", "alpha")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if got != "beta" {
		t.Fatalf("body agent_id should take precedence, got %s", got)
	}

	got, err = reg.Resolve("", "beta")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if got != "beta" {
		t.Fatalf("header agent_id should be used when body missing, got %s", got)
	}

	got, err = reg.Resolve("", "")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if got != "alpha" {
		t.Fatalf("default agent should be used when body/header missing, got %s", got)
	}
}

func TestAgentRegistryUnknownAgentRejected(t *testing.T) {
	cfg := config.DefaultSettings("ActionAgent")
	reg, err := NewAgentRegistry(cfg)
	if err != nil {
		t.Fatalf("new agent registry failed: %v", err)
	}
	if _, err := reg.Resolve("nope", ""); err == nil {
		t.Fatal("expected unknown agent to fail")
	}
}
