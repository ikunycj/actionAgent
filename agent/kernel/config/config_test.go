package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

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
	oldCfg := DefaultSettings("ActionAgent")
	newCfg := oldCfg
	if plan := ClassifyReload(oldCfg, newCfg); plan != ReloadNoop {
		t.Fatalf("expected noop, got %s", plan)
	}
	newCfg.LogLevel = "debug"
	if plan := ClassifyReload(oldCfg, newCfg); plan != ReloadHot {
		t.Fatalf("expected hot, got %s", plan)
	}
	newCfg = oldCfg
	newCfg.HTTPAddr = "127.0.0.1:9999"
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
