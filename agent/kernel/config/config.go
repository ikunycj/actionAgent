package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ConfigFileName = "actionAgent.json"
)

type Source string

const (
	SourceCLI       Source = "cli"
	SourceEnv       Source = "env"
	SourceBinaryDir Source = "binary-dir"
	SourceSystem    Source = "system-default"
)

type Settings struct {
	AppName          string `json:"app_name"`
	HTTPAddr         string `json:"http_addr"`
	LogLevel         string `json:"log_level"`
	EnableRelay      bool   `json:"enable_relay"`
	EnableWSBridge   bool   `json:"enable_ws_bridge"`
	QueueConcurrency int    `json:"queue_concurrency"`
	DedupeTTLSeconds int    `json:"dedupe_ttl_seconds"`
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

func DefaultSettings(appName string) Settings {
	return Settings{
		AppName:          appName,
		HTTPAddr:         "127.0.0.1:8787",
		LogLevel:         "info",
		EnableRelay:      true,
		EnableWSBridge:   true,
		QueueConcurrency: 4,
		DedupeTTLSeconds: 300,
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
	if strings.TrimSpace(cfg.AppName) == "" {
		return errors.New("app_name is required")
	}
	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return errors.New("http_addr is required")
	}
	if cfg.QueueConcurrency < 1 {
		return errors.New("queue_concurrency must be >= 1")
	}
	if cfg.DedupeTTLSeconds < 1 {
		return errors.New("dedupe_ttl_seconds must be >= 1")
	}
	return nil
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
	if oldCfg == newCfg {
		return ReloadNoop
	}
	if oldCfg.HTTPAddr != newCfg.HTTPAddr {
		return ReloadRestart
	}
	return ReloadHot
}
