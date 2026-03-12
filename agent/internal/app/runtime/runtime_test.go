package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"actionagent/agent/internal/core/dispatch"
	"actionagent/agent/internal/core/task"
	"actionagent/agent/internal/platform/config"
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

func portOf(t *testing.T, addr string) int {
	t.Helper()
	_, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port failed for %s: %v", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse port failed for %s: %v", addr, err)
	}
	return port
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
	mustStatus(t, client, http.MethodPost, "http://"+addr+"/v1/responses", map[string]any{"model": "test", "input": "hi"}, http.StatusOK)
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

func TestCLIHTTPAddrOverridesConfig(t *testing.T) {
	tmp := t.TempDir()
	configAddr := freeAddr(t)
	overrideAddr := freeAddr(t)
	cfg := config.DefaultSettings()
	cfg.Port = portOf(t, configAddr)
	cfgPath := filepath.Join(tmp, "cfg.json")
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	rt := NewRuntime(StartOptions{
		CLIConfigPath: cfgPath,
		BinaryPath:    filepath.Join(tmp, "actionagentd"),
		AppName:       "ActionAgent",
		HTTPAddr:      overrideAddr,
	})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+overrideAddr+"/healthz")
	if _, err := client.Get("http://" + configAddr + "/healthz"); err == nil {
		t.Fatalf("expected config addr %s to remain unused", configAddr)
	}
}

func TestAtomicSavePermissionFailurePath(t *testing.T) {
	err := config.AtomicSave(`Z:\__non_existing_drive__\ActionAgent\cfg.json`, config.DefaultSettings())
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
	raw, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "\"app_name\"") || strings.Contains(text, "\"http_addr\"") {
		t.Fatalf("expected generated config to omit app_name/http_addr, got %s", text)
	}
	if !strings.Contains(text, "\"port\"") {
		t.Fatalf("expected generated config to include port, got %s", text)
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

func TestWSFrameControlMethods(t *testing.T) {
	addr := freeAddr(t)
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: addr})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())
	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	runRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "run-1", "method": "agent.run", "params": map[string]any{"a": 1}, "session_id": "s1",
	})
	ok, _ := runRes["ok"].(bool)
	if !ok {
		t.Fatalf("agent.run failed: %+v", runRes)
	}
	payload, _ := runRes["payload"].(map[string]any)
	taskID, _ := payload["task_id"].(string)
	if taskID == "" {
		t.Fatalf("missing task_id: %+v", runRes)
	}

	getRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "get-1", "method": "task.get", "params": map[string]any{"task_id": taskID},
	})
	if ok, _ := getRes["ok"].(bool); !ok {
		t.Fatalf("task.get failed: %+v", getRes)
	}

	waitRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "wait-1", "method": "agent.wait", "params": map[string]any{"task_id": taskID, "timeout_ms": 500},
	})
	if ok, _ := waitRes["ok"].(bool); !ok {
		t.Fatalf("agent.wait failed: %+v", waitRes)
	}

	listRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "list-1", "method": "task.list", "params": map[string]any{"limit": 10},
	})
	if ok, _ := listRes["ok"].(bool); !ok {
		t.Fatalf("task.list failed: %+v", listRes)
	}

	statsRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "stats-1", "method": "session.stats", "session_id": "s1",
	})
	if ok, _ := statsRes["ok"].(bool); !ok {
		t.Fatalf("session.stats failed: %+v", statsRes)
	}

	maintainRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "maintain-1", "method": "session.maintain", "session_id": "s1",
	})
	if ok, _ := maintainRes["ok"].(bool); !ok {
		t.Fatalf("session.maintain failed: %+v", maintainRes)
	}

	auditRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "audit-1", "method": "audit.query", "params": map[string]any{"limit": 5},
	})
	if ok, _ := auditRes["ok"].(bool); !ok {
		t.Fatalf("audit.query failed: %+v", auditRes)
	}

	approvalRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "approval-1", "method": "approval.list", "params": map[string]any{"limit": 5},
	})
	if ok, _ := approvalRes["ok"].(bool); !ok {
		t.Fatalf("approval.list failed: %+v", approvalRes)
	}

	alertRes := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "alert-1", "method": "observability.alerts",
	})
	if ok, _ := alertRes["ok"].(bool); !ok {
		t.Fatalf("observability.alerts failed: %+v", alertRes)
	}
}

func TestBundledWebUIDelivery(t *testing.T) {
	addr := freeAddr(t)
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "webui")
	if err := os.MkdirAll(filepath.Join(webDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir webui assets failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html><body>bundled-console</body></html>"), 0o644); err != nil {
		t.Fatalf("write index failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "assets", "app.js"), []byte("console.log('bundled');"), 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}

	rt := NewRuntime(StartOptions{
		BinaryPath:     filepath.Join(tmp, "actionagentd"),
		AppName:        "ActionAgent",
		HTTPAddr:       addr,
		WebUIAssetsDir: webDir,
	})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	rootResp, err := client.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("root request failed: %v", err)
	}
	defer rootResp.Body.Close()
	rootBody, _ := io.ReadAll(rootResp.Body)
	if rootResp.StatusCode != http.StatusOK || !strings.Contains(string(rootBody), "bundled-console") {
		t.Fatalf("unexpected root response status=%d body=%s", rootResp.StatusCode, string(rootBody))
	}

	routeResp, err := client.Get("http://" + addr + "/app/agents/default/tasks")
	if err != nil {
		t.Fatalf("spa request failed: %v", err)
	}
	defer routeResp.Body.Close()
	routeBody, _ := io.ReadAll(routeResp.Body)
	if routeResp.StatusCode != http.StatusOK || !strings.Contains(string(routeBody), "bundled-console") {
		t.Fatalf("unexpected spa response status=%d body=%s", routeResp.StatusCode, string(routeBody))
	}

	healthResp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer healthResp.Body.Close()
	if got := healthResp.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected health content-type json, got %q", got)
	}
}

func wsCall(t *testing.T, client *http.Client, url string, frame map[string]any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(frame)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ws call failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(raw))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode ws response failed: %v", err)
	}
	return out
}

func TestTaskQueriesAreAgentScoped(t *testing.T) {
	tmp := t.TempDir()
	addr := freeAddr(t)
	cfg := config.DefaultSettings()
	cfg.Port = portOf(t, addr)
	cfg.DefaultAgent = "alpha"
	cfg.Agents = []config.AgentSettings{
		{AgentID: "alpha", Enabled: true, ModelProfile: "openai-main"},
		{AgentID: "beta", Enabled: true, ModelProfile: "openai-main"},
	}
	cfgPath := filepath.Join(tmp, "cfg-agent-scope.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	runTask := func(body map[string]any) string {
		raw, _ := json.Marshal(body)
		resp, err := client.Post("http://"+addr+"/v1/run", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("run request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			payload, _ := io.ReadAll(resp.Body)
			t.Fatalf("unexpected run status=%d body=%s", resp.StatusCode, string(payload))
		}
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode run response failed: %v", err)
		}
		taskID, _ := out["task_id"].(string)
		if taskID == "" {
			t.Fatalf("missing task_id in run response: %+v", out)
		}
		return taskID
	}

	alphaTaskID := runTask(map[string]any{"input": map[string]any{"text": "alpha"}})
	betaTaskID := runTask(map[string]any{"agent_id": "beta", "input": map[string]any{"text": "beta"}})

	alphaList := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "list-alpha", "method": "task.list", "params": map[string]any{"agent_id": "alpha", "limit": 10},
	})
	if ok, _ := alphaList["ok"].(bool); !ok {
		t.Fatalf("task.list alpha failed: %+v", alphaList)
	}
	alphaPayload, _ := alphaList["payload"].(map[string]any)
	alphaTasks, _ := alphaPayload["tasks"].([]any)
	if len(alphaTasks) != 1 {
		t.Fatalf("expected one alpha task, got %+v", alphaList)
	}
	alphaTask, _ := alphaTasks[0].(map[string]any)
	if got, _ := alphaTask["agent_id"].(string); got != "alpha" {
		t.Fatalf("expected alpha task scope, got %+v", alphaTask)
	}

	betaGet := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "get-beta-miss", "method": "task.get", "params": map[string]any{"agent_id": "alpha", "task_id": betaTaskID},
	})
	if ok, _ := betaGet["ok"].(bool); ok {
		t.Fatalf("expected agent-scoped task.get miss, got %+v", betaGet)
	}

	betaList := wsCall(t, client, fmt.Sprintf("http://%s/ws/frame", addr), map[string]any{
		"type": "req", "id": "list-beta", "method": "task.list", "params": map[string]any{"agent_id": "beta", "limit": 10},
	})
	if ok, _ := betaList["ok"].(bool); !ok {
		t.Fatalf("task.list beta failed: %+v", betaList)
	}
	betaPayload, _ := betaList["payload"].(map[string]any)
	betaTasks, _ := betaPayload["tasks"].([]any)
	if len(betaTasks) != 1 {
		t.Fatalf("expected one beta task, got %+v", betaList)
	}
	betaTask, _ := betaTasks[0].(map[string]any)
	if got, _ := betaTask["task_id"].(string); got != betaTaskID {
		t.Fatalf("expected beta task %s, got %+v", betaTaskID, betaTask)
	}
	if alphaTaskID == betaTaskID {
		t.Fatalf("expected distinct task ids for scoped runs")
	}
}

func TestAlertsEndpoint(t *testing.T) {
	addr := freeAddr(t)
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: addr})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())
	rt.metrics.SetQueueDepth(20)
	rt.metrics.SetNodeOnline(0)
	rt.metrics.IncApprovalTimeout()
	rt.metrics.IncApprovalTimeout()
	rt.metrics.IncApprovalTimeout()
	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")
	resp, err := client.Get("http://" + addr + "/alerts")
	if err != nil {
		t.Fatalf("alerts request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected alerts status: %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode alerts failed: %v", err)
	}
	count, _ := body["count"].(float64)
	if int(count) < 1 {
		t.Fatalf("expected alerts, got %v", body)
	}
}

func TestRegressionMatrixConcurrentRequests(t *testing.T) {
	addr := freeAddr(t)
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: addr})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())
	client := &http.Client{Timeout: 3 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	var wg sync.WaitGroup
	errCh := make(chan error, 32)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			payload := map[string]any{
				"input":       map[string]any{"n": n},
				"session_key": fmt.Sprintf("s-%d", n),
			}
			b, _ := json.Marshal(payload)
			req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/run", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				raw, _ := io.ReadAll(resp.Body)
				errCh <- fmt.Errorf("status=%d body=%s", resp.StatusCode, string(raw))
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent request failed: %v", err)
		}
	}
}

func TestRegressionMatrixLongTask(t *testing.T) {
	addr := freeAddr(t)
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: addr})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())
	client := &http.Client{Timeout: 3 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	begin := time.Now()
	mustStatus(t, client, http.MethodPost, "http://"+addr+"/v1/run", map[string]any{
		"input": map[string]any{"sleep_ms": 120},
	}, http.StatusOK)
	if elapsed := time.Since(begin); elapsed < 100*time.Millisecond {
		t.Fatalf("expected long task latency >=100ms, got %s", elapsed)
	}
}

func TestRegressionMatrixNodeFluctuation(t *testing.T) {
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: freeAddr(t)})
	if err := rt.Init(context.Background()); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	now := time.Now()
	rt.dispatch.SetNodes([]dispatch.Node{
		{ID: "local-stale", Local: true, Healthy: true, LastHeartbeat: now.Add(-30 * time.Second), Capabilities: map[string]bool{"run": true}},
		{ID: "remote-ok", Local: false, Healthy: true, LastHeartbeat: now, Capabilities: map[string]bool{"run": true}},
	})
	out, err := rt.Run(context.Background(), task.ExecutionEnvelope{
		TaskID: "t-node", RunID: "r-node", Lane: "main", Operation: "run", Input: map[string]any{"x": 1}, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out.NodeID != "remote-ok" {
		t.Fatalf("expected remote fallback node, got %s", out.NodeID)
	}
}

func TestRegressionMatrixConfigHotUpdate(t *testing.T) {
	rt := NewRuntime(StartOptions{BinaryPath: filepath.Join(t.TempDir(), "actionagentd"), AppName: "ActionAgent", HTTPAddr: freeAddr(t)})
	if err := rt.Init(context.Background()); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	newCfg := rt.cfg
	newCfg.LogLevel = "debug"
	plan, err := rt.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}
	if plan != config.ReloadHot {
		t.Fatalf("expected hot reload plan, got %s", plan)
	}
	out, err := rt.Run(context.Background(), task.ExecutionEnvelope{
		TaskID: "t-hot", RunID: "r-hot", Lane: "main", Operation: "run", Input: map[string]any{"x": 1}, CreatedAt: time.Now(),
	})
	if err != nil || out.State != task.StateSucceeded {
		t.Fatalf("run after hot update failed: out=%+v err=%v", out, err)
	}
}

func TestOpenAIProviderConfigIntegration(t *testing.T) {
	t.Setenv("OPENAI_MOCK_KEY", "k-mock")
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k-mock" {
			t.Fatalf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-mock",
			"choices": []any{
				map[string]any{
					"message": map[string]any{"content": "mock-provider-ok"},
				},
			},
		})
	}))
	defer mock.Close()

	tmp := t.TempDir()
	cfg := config.DefaultSettings()
	addr := freeAddr(t)
	cfg.Port = portOf(t, addr)
	cfg.ModelGateway = config.ModelGatewaySettings{
		Primary: "mock-openai",
		Providers: []config.ProviderSettings{
			{
				Name:          "mock-openai",
				APIStyle:      "openai",
				BaseURL:       mock.URL + "/v1",
				APIKeyEnv:     "OPENAI_MOCK_KEY",
				DefaultModel:  "gpt-mock",
				TimeoutMillis: 3000,
				MaxAttempts:   1,
				Enabled:       true,
			},
		},
	}
	cfg.DefaultAgent = "default"
	cfg.Agents = []config.AgentSettings{
		{AgentID: "default", Enabled: true, ModelProfile: "mock-openai"},
	}
	cfgPath := filepath.Join(tmp, "cfg.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")
	resp, err := client.Post("http://"+addr+"/v1/chat/completions", "application/json", bytes.NewReader([]byte(`{"model":"gpt-mock","messages":[{"role":"user","content":"hi"}]}`)))
	if err != nil {
		t.Fatalf("chat call failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status=%d body=%s", resp.StatusCode, string(raw))
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	choices, _ := body["choices"].([]any)
	if len(choices) == 0 {
		t.Fatalf("expected choices in response: %+v", body)
	}
	msg, _ := choices[0].(map[string]any)
	message, _ := msg["message"].(map[string]any)
	content, _ := message["content"].(string)
	if content != "mock-provider-ok" {
		t.Fatalf("unexpected assistant content: %q", content)
	}
}

func TestProviderWithoutCredentialFallsBackToStatic(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultSettings()
	addr := freeAddr(t)
	cfg.Port = portOf(t, addr)
	cfg.ModelGateway = config.ModelGatewaySettings{
		Primary: "mock-openai",
		Providers: []config.ProviderSettings{
			{
				Name:          "mock-openai",
				APIStyle:      "openai",
				BaseURL:       "http://127.0.0.1:1/v1",
				APIKeyEnv:     "NON_EXISTING_ENV",
				DefaultModel:  "gpt-mock",
				TimeoutMillis: 3000,
				MaxAttempts:   1,
				Enabled:       true,
			},
		},
	}
	cfg.DefaultAgent = "default"
	cfg.Agents = []config.AgentSettings{
		{AgentID: "default", Enabled: true, ModelProfile: "mock-openai"},
	}
	cfgPath := filepath.Join(tmp, "cfg.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")
	resp, err := client.Post("http://"+addr+"/v1/run", "application/json", bytes.NewReader([]byte(`{"input":{"text":"hello"}}`)))
	if err != nil {
		t.Fatalf("run call failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status=%d body=%s", resp.StatusCode, string(raw))
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	payload, _ := body["payload"].(map[string]any)
	provider, _ := payload["provider"].(string)
	if provider != "primary" {
		t.Fatalf("expected static fallback provider=primary, got %q", provider)
	}
}

func TestGatewayAgentIDPrecedence(t *testing.T) {
	tmp := t.TempDir()
	addr := freeAddr(t)
	cfg := config.DefaultSettings()
	cfg.Port = portOf(t, addr)
	cfg.DefaultAgent = "alpha"
	cfg.Agents = []config.AgentSettings{
		{AgentID: "alpha", Enabled: true, ModelProfile: "openai-main"},
		{AgentID: "beta", Enabled: true, ModelProfile: "openai-main"},
	}
	cfgPath := filepath.Join(tmp, "cfg-agent.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	assertRunAgent := func(body map[string]any, headerAgent string, expectedAgent string, expectedStatus int) {
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/run", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if headerAgent != "" {
			req.Header.Set("X-Agent-ID", headerAgent)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != expectedStatus {
			bs, _ := io.ReadAll(resp.Body)
			t.Fatalf("unexpected status=%d body=%s", resp.StatusCode, string(bs))
		}
		if expectedStatus != http.StatusOK {
			return
		}
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		payload, _ := out["payload"].(map[string]any)
		agentID, _ := payload["agent_id"].(string)
		if agentID != expectedAgent {
			t.Fatalf("expected agent_id=%s got %s payload=%+v", expectedAgent, agentID, payload)
		}
	}

	assertRunAgent(map[string]any{"agent_id": "beta", "input": map[string]any{"x": 1}}, "alpha", "beta", http.StatusOK)
	assertRunAgent(map[string]any{"input": map[string]any{"x": 1}}, "beta", "beta", http.StatusOK)
	assertRunAgent(map[string]any{"input": map[string]any{"x": 1}}, "", "alpha", http.StatusOK)
	assertRunAgent(map[string]any{"agent_id": "missing", "input": map[string]any{"x": 1}}, "", "", http.StatusBadRequest)
}

func TestLegacySingleAgentConfigCompatibility(t *testing.T) {
	tmp := t.TempDir()
	addr := freeAddr(t)
	cfg := config.DefaultSettings()
	cfg.Port = portOf(t, addr)
	cfg.DefaultAgent = ""
	cfg.Agents = nil
	cfgPath := filepath.Join(tmp, "cfg-legacy.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	if !rt.legacyCfg {
		t.Fatal("expected legacy config synthesis marker")
	}

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	resp, err := client.Post("http://"+addr+"/v1/run", "application/json", bytes.NewReader([]byte(`{"input":{"text":"hello"}}`)))
	if err != nil {
		t.Fatalf("run call failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status=%d body=%s", resp.StatusCode, string(raw))
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	payload, _ := body["payload"].(map[string]any)
	if got, _ := payload["agent_id"].(string); got != "default" {
		t.Fatalf("expected synthesized default agent_id, got %q payload=%+v", got, payload)
	}
	snap := rt.Metrics()
	if v, ok := snap["config_legacy_agent_synthesized"].(uint64); !ok || v < 1 {
		t.Fatalf("expected legacy synthesis metric >=1, got %v", snap["config_legacy_agent_synthesized"])
	}
}

func TestSharedModelRuntimeAcrossAgents(t *testing.T) {
	t.Setenv("OPENAI_MULTI_KEY", "k-multi")
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-mock",
			"choices": []any{
				map[string]any{
					"message": map[string]any{"content": "ok"},
				},
			},
		})
	}))
	defer mock.Close()

	tmp := t.TempDir()
	addr := freeAddr(t)
	cfg := config.DefaultSettings()
	cfg.Port = portOf(t, addr)
	cfg.ModelGateway = config.ModelGatewaySettings{
		Primary: "mock-openai",
		Providers: []config.ProviderSettings{
			{
				Name:          "mock-openai",
				APIStyle:      "openai",
				BaseURL:       mock.URL + "/v1",
				APIKeyEnv:     "OPENAI_MULTI_KEY",
				DefaultModel:  "gpt-mock",
				TimeoutMillis: 3000,
				MaxAttempts:   1,
				Enabled:       true,
			},
		},
	}
	cfg.DefaultAgent = "alpha"
	cfg.Agents = []config.AgentSettings{
		{AgentID: "alpha", Enabled: true, ModelProfile: "mock-openai"},
		{AgentID: "beta", Enabled: true, ModelProfile: "mock-openai"},
	}
	cfgPath := filepath.Join(tmp, "cfg-multi.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	var wg sync.WaitGroup
	for _, agentID := range []string{"alpha", "beta"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			body := map[string]any{"agent_id": id, "model": "gpt-mock", "messages": []any{map[string]any{"role": "user", "content": "hi"}}}
			raw, _ := json.Marshal(body)
			req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/chat/completions", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("request failed for %s: %v", id, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				bs, _ := io.ReadAll(resp.Body)
				t.Errorf("unexpected status for %s: %d body=%s", id, resp.StatusCode, string(bs))
			}
		}(agentID)
	}
	wg.Wait()

	metrics := rt.Metrics()
	if metrics["model_agent_alpha_route_ok"] == nil || metrics["model_agent_beta_route_ok"] == nil {
		t.Fatalf("expected per-agent route metrics, got %+v", metrics)
	}
}

func TestResponsesStreamProxyIntegration(t *testing.T) {
	t.Setenv("OPENAI_STREAM_KEY", "k-stream")
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k-stream" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("Accept") != "*/*" {
			t.Fatalf("unexpected accept header: %q", r.Header.Get("Accept"))
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		if stream, _ := req["stream"].(bool); !stream {
			t.Fatalf("expected stream=true, got: %+v", req)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: response.output_text.delta\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "event: response.completed\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\"}\n\n")
	}))
	defer mock.Close()

	tmp := t.TempDir()
	addr := freeAddr(t)
	cfg := config.DefaultSettings()
	cfg.Port = portOf(t, addr)
	cfg.ModelGateway = config.ModelGatewaySettings{
		Primary: "mock-openai",
		Providers: []config.ProviderSettings{
			{
				Name:          "mock-openai",
				APIStyle:      "openai",
				BaseURL:       mock.URL + "/v1",
				APIKeyEnv:     "OPENAI_STREAM_KEY",
				DefaultModel:  "gpt-stream",
				TimeoutMillis: 3000,
				MaxAttempts:   1,
				Enabled:       true,
			},
		},
	}
	cfg.DefaultAgent = "default"
	cfg.Agents = []config.AgentSettings{
		{AgentID: "default", Enabled: true, ModelProfile: "mock-openai"},
	}
	cfgPath := filepath.Join(tmp, "cfg-stream.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg failed: %v", err)
	}

	rt := NewRuntime(StartOptions{CLIConfigPath: cfgPath, BinaryPath: filepath.Join(tmp, "actionagentd"), AppName: "ActionAgent"})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer rt.Shutdown(context.Background())

	client := &http.Client{Timeout: 3 * time.Second}
	waitHealthy(t, client, "http://"+addr+"/healthz")

	reqBody := []byte(`{"model":"gpt-stream","input":[{"role":"user","content":"ping"}],"stream":true}`)
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/responses", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status=%d body=%s", resp.StatusCode, string(raw))
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}
	raw, _ := io.ReadAll(resp.Body)
	s := string(raw)
	if !strings.Contains(s, "response.output_text.delta") || !strings.Contains(s, "\"pong\"") || !strings.Contains(s, "response.completed") {
		t.Fatalf("unexpected stream payload: %s", s)
	}
}
