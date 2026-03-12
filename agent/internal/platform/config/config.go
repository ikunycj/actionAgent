package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

const (
	ConfigFileName = "actionAgent.json"
	DefaultPort    = 8000
)

type Source string

const (
	SourceCLI       Source = "cli"
	SourceEnv       Source = "env"
	SourceBinaryDir Source = "binary-dir"
	SourceSystem    Source = "system-default"
)

type Settings struct {
	Port             int                  `json:"port"`
	LogLevel         string               `json:"log_level"`
	EnableRelay      bool                 `json:"enable_relay"`
	EnableWSBridge   bool                 `json:"enable_ws_bridge"`
	QueueConcurrency int                  `json:"queue_concurrency"`
	DedupeTTLSeconds int                  `json:"dedupe_ttl_seconds"`
	DefaultAgent     string               `json:"default_agent,omitempty"`
	Agents           []AgentSettings      `json:"agents,omitempty"`
	ModelGateway     ModelGatewaySettings `json:"model_gateway"`

	LegacyImplicitDefault bool `json:"-"`
}

type ModelGatewaySettings struct {
	Primary   string             `json:"primary"`
	Fallbacks []string           `json:"fallbacks"`
	Providers []ProviderSettings `json:"providers"`
}

type AgentSettings struct {
	AgentID       string `json:"agent_id"`
	Enabled       bool   `json:"enabled"`
	ModelProfile  string `json:"model_profile,omitempty"`
	PromptProfile string `json:"prompt_profile,omitempty"`
	ToolPolicy    string `json:"tool_policy,omitempty"`
	MemoryPolicy  string `json:"memory_policy,omitempty"`
	SessionPolicy string `json:"session_policy,omitempty"`
}

type ProviderSettings struct {
	Name          string `json:"name"`
	APIStyle      string `json:"api_style"`
	BaseURL       string `json:"base_url"`
	APIKeyEnv     string `json:"api_key_env,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	DefaultModel  string `json:"model"`
	TimeoutMillis int    `json:"timeout_ms"`
	MaxAttempts   int    `json:"max_attempts"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
	Enabled       bool   `json:"enabled"`
}

type ResolveInput struct {
	CLIPath     string
	EnvPath     string
	BinaryDir   string
	AppName     string
	GOOS        string
	EnsureExist bool
}

type Resolved struct {
	Path   string
	Source Source
}

func DefaultSettings() Settings {
	return Settings{
		Port:             DefaultPort,
		LogLevel:         "info",
		EnableRelay:      true,
		EnableWSBridge:   true,
		QueueConcurrency: 4,
		DedupeTTLSeconds: 300,
		DefaultAgent:     "default",
		Agents: []AgentSettings{
			{
				AgentID:       "default",
				Enabled:       true,
				ModelProfile:  "openai-main",
				PromptProfile: "default",
				ToolPolicy:    "default",
				MemoryPolicy:  "default",
				SessionPolicy: "default",
			},
		},
		ModelGateway: ModelGatewaySettings{
			Primary:   "openai-main",
			Fallbacks: []string{"anthropic-backup"},
			Providers: []ProviderSettings{
				{
					Name:          "openai-main",
					APIStyle:      "openai",
					BaseURL:       "https://api.openai.com/v1",
					APIKeyEnv:     "ACTIONAGENT_OPENAI_API_KEY",
					DefaultModel:  "gpt-4o-mini",
					TimeoutMillis: 20000,
					MaxAttempts:   2,
					Enabled:       true,
				},
				{
					Name:          "anthropic-backup",
					APIStyle:      "anthropic",
					BaseURL:       "https://api.anthropic.com/v1",
					APIKeyEnv:     "ACTIONAGENT_ANTHROPIC_API_KEY",
					DefaultModel:  "claude-3-5-sonnet-20241022",
					TimeoutMillis: 25000,
					MaxAttempts:   2,
					MaxTokens:     1024,
					Enabled:       true,
				},
			},
		},
	}
}

func ResolvePath(in ResolveInput) (Resolved, error) {
	appName := in.AppName
	if appName == "" {
		appName = "ActionAgent"
	}
	goos := in.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if strings.TrimSpace(in.CLIPath) != "" {
		return Resolved{Path: in.CLIPath, Source: SourceCLI}, nil
	}
	if strings.TrimSpace(in.EnvPath) != "" {
		return Resolved{Path: in.EnvPath, Source: SourceEnv}, nil
	}
	if strings.TrimSpace(in.BinaryDir) != "" {
		p := filepath.Join(in.BinaryDir, ConfigFileName)
		if !in.EnsureExist || fileExists(p) {
			return Resolved{Path: p, Source: SourceBinaryDir}, nil
		}
	}
	for _, p := range systemDefaultCandidates(appName, goos) {
		if !in.EnsureExist || fileExists(p) {
			return Resolved{Path: p, Source: SourceSystem}, nil
		}
	}
	return Resolved{}, errors.New("unable to resolve config path")
}

func systemDefaultCandidates(appName, goos string) []string {
	switch goos {
	case "windows":
		// Keep filename aligned with current design source.
		return []string{filepath.Join(`C:\ProgramData`, appName, "acgtionAgent.json")}
	case "linux":
		return []string{filepath.Join("/etc", strings.ToLower(appName), ConfigFileName)}
	case "darwin":
		return []string{filepath.Join("/etc", strings.ToLower(appName), ConfigFileName)}
	default:
		return []string{filepath.Join(".", ConfigFileName)}
	}
}

func LoadSingleSource(path string) (Settings, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Settings{}, err
	}
	var cfg Settings
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Settings{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	Normalize(&cfg)
	if err := Validate(cfg); err != nil {
		return Settings{}, err
	}
	return cfg, nil
}

func EnsureConfig(path string, defaults Settings) error {
	if fileExists(path) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return AtomicSave(path, defaults)
}

func AtomicSave(path string, cfg Settings) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Validate(cfg Settings) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if cfg.QueueConcurrency < 1 {
		return errors.New("queue_concurrency must be >= 1")
	}
	if cfg.DedupeTTLSeconds < 1 {
		return errors.New("dedupe_ttl_seconds must be >= 1")
	}
	agentsByID := map[string]AgentSettings{}
	enabledAgents := map[string]struct{}{}
	providers := map[string]struct{}{}
	for i, p := range cfg.ModelGateway.Providers {
		if !p.Enabled {
			if name := strings.TrimSpace(p.Name); name != "" {
				providers[name] = struct{}{}
			}
			continue
		}
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("model_gateway.providers[%d].name is required", i)
		}
		providers[strings.TrimSpace(p.Name)] = struct{}{}
		if style := strings.TrimSpace(strings.ToLower(p.APIStyle)); style != "openai" && style != "anthropic" {
			return fmt.Errorf("model_gateway.providers[%d].api_style must be openai|anthropic", i)
		}
		if strings.TrimSpace(p.BaseURL) == "" {
			return fmt.Errorf("model_gateway.providers[%d].base_url is required", i)
		}
		if strings.TrimSpace(p.DefaultModel) == "" {
			return fmt.Errorf("model_gateway.providers[%d].model is required", i)
		}
		if strings.TrimSpace(p.APIKey) == "" && strings.TrimSpace(p.APIKeyEnv) == "" {
			return fmt.Errorf("model_gateway.providers[%d] requires api_key or api_key_env", i)
		}
		if p.TimeoutMillis < 1000 {
			return fmt.Errorf("model_gateway.providers[%d].timeout_ms must be >= 1000", i)
		}
		if p.MaxAttempts < 1 {
			return fmt.Errorf("model_gateway.providers[%d].max_attempts must be >= 1", i)
		}
	}
	if len(cfg.Agents) == 0 {
		return errors.New("agents requires at least one entry")
	}
	for i, a := range cfg.Agents {
		id := strings.TrimSpace(a.AgentID)
		if id == "" {
			return fmt.Errorf("agents[%d].agent_id is required", i)
		}
		if _, ok := agentsByID[id]; ok {
			return fmt.Errorf("duplicate agents[%d].agent_id: %s", i, id)
		}
		agentsByID[id] = a
		if a.Enabled {
			enabledAgents[id] = struct{}{}
		}
		if prof := strings.TrimSpace(a.ModelProfile); prof != "" {
			if _, ok := providers[prof]; !ok {
				return fmt.Errorf("agents[%d].model_profile references unknown provider: %s", i, prof)
			}
		}
	}
	defaultAgent := strings.TrimSpace(cfg.DefaultAgent)
	if defaultAgent == "" {
		return errors.New("default_agent is required")
	}
	if _, ok := enabledAgents[defaultAgent]; !ok {
		return fmt.Errorf("default_agent %q must reference an enabled agent", defaultAgent)
	}
	return nil
}

func Normalize(cfg *Settings) {
	if cfg == nil {
		return
	}
	if cfg.Port <= 0 {
		cfg.Port = DefaultPort
	}
	if strings.TrimSpace(cfg.ModelGateway.Primary) == "" {
		cfg.ModelGateway.Primary = "openai-main"
	}
	if len(cfg.Agents) == 0 {
		cfg.Agents = []AgentSettings{
			{
				AgentID:       "default",
				Enabled:       true,
				ModelProfile:  strings.TrimSpace(cfg.ModelGateway.Primary),
				PromptProfile: "default",
				ToolPolicy:    "default",
				MemoryPolicy:  "default",
				SessionPolicy: "default",
			},
		}
		cfg.DefaultAgent = "default"
		cfg.LegacyImplicitDefault = true
	}
	if strings.TrimSpace(cfg.DefaultAgent) == "" {
		for _, a := range cfg.Agents {
			if a.Enabled && strings.TrimSpace(a.AgentID) != "" {
				cfg.DefaultAgent = strings.TrimSpace(a.AgentID)
				break
			}
		}
	}
	for i := range cfg.Agents {
		cfg.Agents[i].AgentID = strings.TrimSpace(cfg.Agents[i].AgentID)
		if strings.TrimSpace(cfg.Agents[i].ModelProfile) == "" {
			cfg.Agents[i].ModelProfile = strings.TrimSpace(cfg.ModelGateway.Primary)
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type ReloadPlan string

const (
	ReloadNoop    ReloadPlan = "noop"
	ReloadHot     ReloadPlan = "hot"
	ReloadRestart ReloadPlan = "restart"
)

func ClassifyReload(oldCfg, newCfg Settings) ReloadPlan {
	if reflect.DeepEqual(oldCfg, newCfg) {
		return ReloadNoop
	}
	if oldCfg.Port != newCfg.Port {
		return ReloadRestart
	}
	return ReloadHot
}

func ListenAddr(port int) string {
	if port <= 0 {
		port = DefaultPort
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func ParseListenPort(addr string) (int, error) {
	hostPort := strings.TrimSpace(addr)
	if hostPort == "" {
		return 0, errors.New("addr is required")
	}
	_, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}
	return port, nil
}
