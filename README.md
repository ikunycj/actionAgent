# ActionAgent

A deployment-first distributed general-purpose Agent platform.

## Positioning

ActionAgent aims to deliver executable, collaborative, and extensible agent capabilities with minimal deployment overhead.

Core value:
1. Trigger instant tasks from clients or web.
2. Run long tasks continuously on local or remote nodes.
3. Push results back with auditability and traceability.

Chinese version: [README.zh-CN.md](README.zh-CN.md)

## Product Shape

ActionAgent follows a dual-plane architecture: control plane + execution plane.

### 1) Core (Execution Core)
- Form: Go single-binary runtime (`actionagentd`)
- Platforms: Windows / Linux / macOS
- Responsibility: task execution, model calls, tool runtime, logs, and audit output
- Current status: available in MVP (local single-node runtime)

### 2) Client (Control Plane)
- Form: desktop / mobile client (phased)
- Responsibility: trigger tasks, view status, handle approvals, receive receipts
- Current status: currently exposed mainly via HTTP API

### 3) Cloud Relay (Optional)
- Responsibility: cross-network node relay and collaboration
- Current status: planned

### 4) Team Console (Later Stage)
- Responsibility: org permissions, policy templates, audit center, node orchestration
- Current status: planned

## Current MVP Capabilities
1. Single-process runtime (`actionagentd`)
2. Health check (`GET /healthz`)
3. OpenAI-compatible endpoint (`POST /v1/chat/completions`)
4. Direct run endpoint (`POST /v1/run`)
5. Env-first config with optional `config.json`

## How To Use

### A. Local Quick Start (Recommended)

1. Build

```bash
go build -o actionagentd ./cmd/actionagentd
```

2. Run with minimum env

```bash
ACTIONAGENT_API_KEY=sk-xxx ./actionagentd
```

3. Health check

```bash
curl http://127.0.0.1:8787/healthz
```

### B. OpenAI-Compatible Usage

```bash
curl -X POST http://127.0.0.1:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

### C. Direct Task Execution

```bash
curl -X POST http://127.0.0.1:8787/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "input":"Summarize this paragraph in Chinese.",
    "model":"gpt-4o-mini"
  }'
```

Note: `/v1/run` request fields should follow the current server implementation.

## Typical Scenarios
1. Instant task: one-line trigger, fast response
2. Long task: backend/scheduled trigger with async completion
3. Cross-device relay (planned): trigger on phone, execute on desktop/cloud

## Configuration

Optional config file: `config.json` (see `config.example.json`).

Environment variables (override config file):
- `ACTIONAGENT_ADDR` (default `127.0.0.1:8787`)
- `ACTIONAGENT_UPSTREAM_BASE_URL` (default `https://api.openai.com/v1`)
- `ACTIONAGENT_API_KEY`
- `ACTIONAGENT_DEFAULT_MODEL` (default `gpt-4o-mini`)
- `ACTIONAGENT_REQUEST_TIMEOUT_SECONDS` (default `120`)
- `ACTIONAGENT_SYSTEM_PROMPT`

Legacy compatibility:
- `GOCLAW_*` variables are still supported.

## Commit Message Policy

Commit messages must be English-only (ASCII).

This repository enforces it via:
1. Local git hook: `.githooks/commit-msg`
2. CI workflow: `.github/workflows/commit-message-english.yml`

Enable local hook path:

```powershell
powershell -ExecutionPolicy Bypass -File ./scripts/setup-hooks.ps1
```

## Deployment Notes
1. Dev: env-only startup for fast validation
2. Prod: fixed config + process supervisor (systemd / launchd / Windows Service)
3. Security: keep API keys in secure storage, never commit secrets

## Documentation
- Product planning: `docs/actionagent-design.md`
- Agent kernel PRD: `docs/design/agent-kernel-product-design.md`
- Agent kernel technical solution: `docs/design/agent-kernel-technical-solution.md`

## Roadmap Note
This repository is in MVP evolution. Distributed relay, approval flow, and team governance will be released in phases.
