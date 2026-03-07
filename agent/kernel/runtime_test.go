package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"actionagent/agent/kernel/config"
	"actionagent/agent/kernel/task"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func TestStartupHealthAndMinimalAPI(t *testing.T) {
	addr := freeAddr(t)
	tmp := t.TempDir()
	rt := NewRuntime(StartOptions{
		BinaryPath: filepath.Join(tmp, "actionagentd"),
		AppName:    "ActionAgent",
		HTTPAddr:   addr,
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	mustStatus(t, client, http.MethodPost, "http://"+addr+"/v1/run", map[string]any{"input": map[string]any{"x": 1}}, http.StatusOK)
	mustStatus(t, client, http.MethodPost, "http://"+addr+"/v1/chat/completions", map[string]any{"model": "test", "messages": []any{"hi"}}, http.StatusOK)
}

func TestRelayTerminalConvergence(t *testing.T) {
	tmp := t.TempDir()
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent", HTTPAddr: freeAddr(t)})
	if err := rt.Init(context.Background()); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	env1 := task.ExecutionEnvelope{TaskID: "task-fixed", RunID: "run-1", Lane: "main", Operation: "run", Input: map[string]any{"a": 1}}
	env2 := task.ExecutionEnvelope{TaskID: "task-fixed", RunID: "run-2", Lane: "main", Operation: "run", Input: map[string]any{"a": 2}}
	out1, err := rt.Run(context.Background(), env1)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	out2, err := rt.Run(context.Background(), env2)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if out1.RunID != out2.RunID {
		t.Fatalf("expected terminal convergence, got %s and %s", out1.RunID, out2.RunID)
	}
}

func TestStartupFailFast(t *testing.T) {
	rt := NewRuntime(StartOptions{AppName: "ActionAgent", BinaryPath: filepath.Join(t.TempDir(), "bin"), InitFailures: map[string]error{"logging": errors.New("boom")}})
	if err := rt.Start(context.Background()); err == nil {
		t.Fatal("expected startup failure")
	}
}

func TestInvalidConfigFailsStartup(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(cfg, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	rt := NewRuntime(StartOptions{CLIConfigPath: cfg, BinaryPath: filepath.Join(tmp, "bin"), AppName: "ActionAgent", HTTPAddr: freeAddr(t)})
	if err := rt.Start(context.Background()); err == nil {
		t.Fatal("expected startup failure for invalid config")
	}
}

func TestAtomicSavePermissionFailurePath(t *testing.T) {
	err := config.AtomicSave(`Z:\__non_existing_drive__\ActionAgent\cfg.json`, config.DefaultSettings("ActionAgent"))
	if err == nil {
		t.Fatal("expected atomic save failure on invalid path")
	}
}

func waitHealthy(t *testing.T, client *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("health never became ready: %s", url)
}

func mustStatus(t *testing.T, client *http.Client, method, url string, body map[string]any, status int) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		bs, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d, body=%s", resp.StatusCode, string(bs))
	}
}

func TestConfigInitCreatesFileInBinaryDir(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "actionagentd")
	rt := NewRuntime(StartOptions{BinaryPath: binPath, AppName: "ActionAgent", HTTPAddr: freeAddr(t)})
	if err := rt.Init(context.Background()); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	expected := filepath.Join(tmp, "actionAgent.json")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected config auto-created at %s: %v", expected, err)
	}
	if rt.cfgPath != expected {
		t.Fatalf("expected cfg path %s, got %s", expected, rt.cfgPath)
	}
}

func TestWSFrameEndpointValidation(t *testing.T) {
	addr := freeAddr(t)
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: addr})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())
	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	body := map[string]any{"type": "req", "id": "1", "method": "agent.run", "params": map[string]any{"a": 1}, "connection_id": "c1", "session_id": "s1"}
	mustStatus(t, client, http.MethodPost, fmt.Sprintf("http://%s/ws/frame", addr), body, http.StatusOK)
}
