# ActionAgent

A deployment-first distributed Agent platform focused on executable tasks, observability, and auditable operations.

Chinese version: [README.zh-CN.md](README.zh-CN.md)

## 1. Project Overview

### Positioning

ActionAgent aims to provide a low-friction Agent runtime that can be started quickly, run reliably, and remain operationally observable.

Core value:
1. Trigger instant tasks from clients or web entrypoints.
2. Run long tasks continuously on local or remote nodes.
3. Return results with traceable logs and audit records.

### Macro Architecture

ActionAgent follows a dual-plane model: control plane + execution plane.

1. Core (Execution Core)
- Form: Go single-binary runtime (`actionagentd`)
- Platforms: Windows / Linux / macOS
- Responsibility: task execution, model routing, tool runtime, logging, events, and audit output

2. Client (Control Plane)
- Form: desktop/mobile clients (phased rollout)
- Responsibility: trigger tasks, view status, process approvals, receive receipts

3. Cloud Relay (Optional)
- Responsibility: cross-network relay and collaboration between nodes

4. Team Console (Later Stage)
- Responsibility: org governance, policy templates, audit center, and node orchestration

### Current MVP Scope

1. Single-process runtime (`actionagentd`)
2. Health check (`GET /healthz`)
3. OpenAI-compatible endpoint (`POST /v1/chat/completions`)
4. Direct run endpoint (`POST /v1/run`)
5. Typed frame bridge endpoint (`POST /ws/frame`)
6. Baseline event stream and metrics output

### Current Implementation Snapshot (2026-03-08)

The kernel has reached an initial runnable shape, with these implemented modules:
1. Config loading and source resolution (`--config` / env / binary-dir / system default)
2. Task engine with lane-aware queue, state transitions, and idempotency dedupe
3. Dispatcher and terminal aggregation path for request lifecycle closure
4. Model gateway with primary + fallback routing
5. Tool runtime with tiered permissions and approval-token checks
6. Session transcript + memory retrieval fallback
7. Event bus and metrics endpoint output

Still planned for next phases:
1. Multi-node relay hardening and richer recovery snapshots
2. Production-grade approval workflow and persistence
3. WebUI and team-governance capabilities

### Roadmap Snapshot

The repository remains in active MVP evolution with a working kernel baseline in place.

## 2. How to Use

### Prerequisites

1. Go 1.25+ (recommended: Go 1.25.8)
2. Windows/Linux/macOS shell environment

### Local Quick Start

From repository root:

1. Build

```bash
cd agent
go build -o actionagentd ./cmd/actionagentd
```

2. Run with explicit config path (recommended)

```bash
./actionagentd --config "$(pwd)/actionAgent.json"
```

3. Health check

```bash
curl http://127.0.0.1:8000/healthz
```

### API Usage Examples

1. OpenAI-compatible call

```bash
curl -X POST http://127.0.0.1:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

2. Direct run call

```bash
curl -X POST http://127.0.0.1:8000/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "input":{"text":"Summarize this paragraph in Chinese."}
  }'
```

### Configuration Rules

Config path resolution order:
1. `--config`
2. `ACTIONAGENT_CONFIG`
3. `<binary-dir>/actionAgent.json`
4. System defaults (lower priority than binary-dir)
- Linux: `/etc/<appname>/actionAgent.json`
- Windows: `C:\ProgramData\<AppName>\acgtionAgent.json`

Runtime behavior:
1. The runtime loads exactly one resolved config file.
2. Field-level multi-source merge is not applied.
3. If the resolved file does not exist and path is writable, default config is auto-created.

Model gateway (stage-1 complete):
1. Configure providers under `model_gateway.providers`.
2. Configure agent instances under `agents` and set `default_agent`.
3. Supported `api_style`: `openai` and `anthropic`.
4. Recommended key env vars:
   - `ACTIONAGENT_OPENAI_API_KEY`
   - `ACTIONAGENT_ANTHROPIC_API_KEY`
5. Reference design doc:
   - `docs/design/agent-model-provider-configuration.md`

### Multi-Agent Migration (Rollout / Rollback)

Rollout:
1. Upgrade binary to the version with multi-agent support.
2. Change `http_addr` to `127.0.0.1:8000` (or pass `--addr` override).
3. Add `default_agent` and `agents` to config; keep one `default` agent first.
4. Gradually route traffic with `agent_id` or `X-Agent-ID`; requests without agent selector still route to `default_agent`.
5. Verify metrics include `model_agent_<id>_*` counters.

Rollback:
1. Stop the new binary and restart the previous stable binary.
2. Restore previous config file backup.
3. If needed, remove `agent_id` headers/body fields from client requests and route through legacy single-agent flow.

### Deployment Helper Scripts

1. PowerShell: `./scripts/start-agent.ps1`
2. Bash: `./scripts/start-agent.sh`
3. Model path verify (PowerShell): `./scripts/verify-model-provider.ps1`
4. Model path verify (Bash): `./scripts/verify-model-provider.sh`

## 3. Development Guide

### Repository Structure

1. `agent/`: Agent kernel runtime implementation (Go)
2. `docs/`: product/technical design and reference docs
3. `openspec/`: change proposals, specs, design, and task tracking
4. `scripts/`: local development and startup helper scripts

### Build and Test

From `agent/`:

```bash
go test ./...
```

### Recommended Development Workflow

1. Confirm product and technical intent in `docs/design/`.
2. Create or update a change in OpenSpec (`/opsx:propose`).
3. Implement tasks using `/opsx:apply` and keep task checkboxes in sync.
4. Run tests (`go test ./...`) before review.
5. Archive completed changes with `/opsx:archive <change-name>`.

### Contribution and Quality Policy

1. Commit messages must be English-only (ASCII).
2. Enable local commit hook:

```powershell
powershell -ExecutionPolicy Bypass -File ./scripts/setup-hooks.ps1
```

3. Keep code changes scoped to the active OpenSpec tasks.

### Related Documents

1. Product planning: `docs/actionagent-design.md`
2. Agent kernel PRD: `docs/design/agent-kernel-product-design.md`
3. Agent kernel technical solution: `docs/design/agent-kernel-technical-solution.md`
