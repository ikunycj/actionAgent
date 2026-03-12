package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultSettingsPortAndAgents(t *testing.T) {
	cfg := DefaultSettings()
	if cfg.Port != DefaultPort {
		t.Fatalf("expected default port %d, got %d", DefaultPort, cfg.Port)
	}
	if cfg.DefaultAgent == "" || len(cfg.Agents) != 1 || cfg.Agents[0].AgentID != "default" {
		t.Fatalf("expected default single-agent bootstrap, got default_agent=%q agents=%+v", cfg.DefaultAgent, cfg.Agents)
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	in := ResolveInput{
		CLIPath:   "/tmp/cli.json",
		EnvPath:   "/tmp/env.json",
		BinaryDir: "/tmp/bin",
		AppName:   "ActionAgent",
		GOOS:      "linux",
	}
	res, err := ResolvePath(in)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if res.Source != SourceCLI || res.Path != "/tmp/cli.json" {
		t.Fatalf("expected cli precedence, got %+v", res)
	}

	in.CLIPath = ""
	res, _ = ResolvePath(in)
	if res.Source != SourceEnv {
		t.Fatalf("expected env precedence, got %+v", res)
	}

	in.EnvPath = ""
	res, _ = ResolvePath(in)
	if res.Source != SourceBinaryDir || res.Path != filepath.Join("/tmp/bin", ConfigFileName) {
		t.Fatalf("expected binary-dir precedence, got %+v", res)
	}
}

func TestClassifyReload(t *testing.T) {
	oldCfg := DefaultSettings()
	newCfg := oldCfg
	if plan := ClassifyReload(oldCfg, newCfg); plan != ReloadNoop {
		t.Fatalf("expected noop, got %s", plan)
	}
	newCfg.LogLevel = "debug"
	if plan := ClassifyReload(oldCfg, newCfg); plan != ReloadHot {
		t.Fatalf("expected hot, got %s", plan)
	}
	newCfg = oldCfg
	newCfg.Port = 9999
	if plan := ClassifyReload(oldCfg, newCfg); plan != ReloadRestart {
		t.Fatalf("expected restart, got %s", plan)
	}
}

func TestSystemDefaultCandidates(t *testing.T) {
	res, err := ResolvePath(ResolveInput{AppName: "ActionAgent", GOOS: runtime.GOOS, EnsureExist: false})
	if err != nil {
		t.Fatalf("resolve should return system fallback candidate: %v", err)
	}
	if res.Source != SourceSystem {
		t.Fatalf("expected system fallback, got %s", res.Source)
	}
}

func TestValidateProviderSettings(t *testing.T) {
	cfg := DefaultSettings()
	if err := Validate(cfg); err != nil {
		t.Fatalf("default settings should be valid: %v", err)
	}

	cfg.ModelGateway.Providers[0].APIStyle = "unknown"
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "api_style") {
		t.Fatalf("expected api_style validation error, got %v", err)
	}
}

func TestValidatePortRange(t *testing.T) {
	cfg := DefaultSettings()
	cfg.Port = 0
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("expected port validation error, got %v", err)
	}
}

func TestNormalizeLegacyConfigSynthesizesDefaultAgent(t *testing.T) {
	cfg := DefaultSettings()
	cfg.Agents = nil
	cfg.DefaultAgent = ""
	cfg.LegacyImplicitDefault = false

	Normalize(&cfg)
	if !cfg.LegacyImplicitDefault {
		t.Fatal("expected legacy synthesis marker")
	}
	if cfg.DefaultAgent != "default" {
		t.Fatalf("expected synthesized default agent id, got %q", cfg.DefaultAgent)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].AgentID != "default" {
		t.Fatalf("expected synthesized default agent entry, got %+v", cfg.Agents)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("synthesized config should validate: %v", err)
	}
}

func TestValidateAgentModelProfileAndIdentity(t *testing.T) {
	cfg := DefaultSettings()
	cfg.Agents = append(cfg.Agents, AgentSettings{AgentID: "default", Enabled: true, ModelProfile: "openai-main"})
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate agent id validation error, got %v", err)
	}

	cfg = DefaultSettings()
	cfg.DefaultAgent = "missing"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "default_agent") {
		t.Fatalf("expected default_agent validation error, got %v", err)
	}

	cfg = DefaultSettings()
	cfg.Agents[0].ModelProfile = "not-exist-provider"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "model_profile") {
		t.Fatalf("expected model_profile validation error, got %v", err)
	}
}

func TestListenAddrAndParseListenPort(t *testing.T) {
	if got := ListenAddr(8123); got != "127.0.0.1:8123" {
		t.Fatalf("unexpected listen addr: %s", got)
	}
	port, err := ParseListenPort("0.0.0.0:9123")
	if err != nil {
		t.Fatalf("parse listen port failed: %v", err)
	}
	if port != 9123 {
		t.Fatalf("expected port 9123, got %d", port)
	}
}

func TestLoadSingleSourceIgnoresLegacyAppNameAndHTTPAddr(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.json")
	raw := `{
  "app_name": "ActionAgent",
  "http_addr": "0.0.0.0:9999",
  "log_level": "info",
  "enable_relay": true,
  "enable_ws_bridge": true,
  "queue_concurrency": 4,
  "dedupe_ttl_seconds": 300,
  "default_agent": "default",
  "agents": [{"agent_id":"default","enabled":true,"model_profile":"openai-main"}],
  "model_gateway": {
    "primary": "openai-main",
    "fallbacks": [],
    "providers": [{
      "name":"openai-main",
      "api_style":"openai",
      "base_url":"https://api.openai.com/v1",
      "api_key":"x",
      "model":"gpt-4o-mini",
      "timeout_ms":20000,
      "max_attempts":2,
      "enabled":true
    }]
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	cfg, err := LoadSingleSource(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("expected legacy config to normalize to default port %d, got %d", DefaultPort, cfg.Port)
	}
}
