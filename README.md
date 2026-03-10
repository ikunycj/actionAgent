# ActionAgent - Deployment-first Agent Runtime

A deployment-first distributed Agent runtime focused on executable tasks, observability, and auditable operations.

Chinese version: [README.zh-CN.md](README.zh-CN.md)

[Quick Start](#quick-start-tldr) | [API Surface](#api-surface) | [Configuration](#configuration) | [Development Workflow](#development-workflow) | [Roadmap](#roadmap)

ActionAgent follows a dual-plane model (control plane + execution plane): clients or web entrypoints trigger tasks, and `actionagentd` executes them with traceable logs, metrics, events, and audit records.

## Highlights

- Single-binary runtime (`actionagentd`) for Windows/Linux/macOS.
- OpenAI-compatible APIs: `POST /v1/chat/completions` and `POST /v1/responses`.
- Direct task API: `POST /v1/run` with lane/session/idempotency support.
- Typed bridge API: `POST /ws/frame` for req/res/event integrations.
- Runtime observability: `GET /healthz`, `GET /events`, `GET /metrics`, `GET /alerts`.
- Multi-agent routing with deterministic selector priority: `body.agent_id` > `X-Agent-ID` > `default_agent`.
- Model gateway with `primary + fallbacks` and provider adapters (`openai`, `anthropic`).

## Quick Start (TL;DR)

Runtime requirement: Go `1.25+` (repo toolchain: `go1.25.8`).

```bash
cd agent
go build -o actionagentd ./cmd/actionagentd
./actionagentd --config "$(pwd)/actionAgent.json"
```

PowerShell:

```powershell
cd agent
go build -o actionagentd.exe ./cmd/actionagentd
.\actionagentd.exe --config "$PWD\actionAgent.json"
```

Health check:

```bash
curl http://127.0.0.1:8000/healthz
```

## API Surface

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/healthz` | `GET` | liveness/readiness |
| `/v1/run` | `POST` | generic task execution |
| `/v1/chat/completions` | `POST` | OpenAI Chat Completions compatible |
| `/v1/responses` | `POST` | OpenAI Responses style API (supports stream passthrough) |
| `/ws/frame` | `POST` | typed frame request/response bridge |
| `/events` | `GET` | realtime event stream (JSON lines, not SSE) |
| `/metrics` | `GET` | runtime metrics snapshot |
| `/alerts` | `GET` | alert evaluation output |

Chat Completions example:

```bash
curl -X POST http://127.0.0.1:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

Direct run example:

```bash
curl -X POST http://127.0.0.1:8000/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "input":{"text":"Summarize this paragraph in Chinese."}
  }'
```

## How It Works (Short)

```text
Client / CLI / Future UI
           |
           v
      HTTP Gateway
(/v1/run, /v1/chat/completions, /v1/responses, /ws/frame)
           |
           v
 Task Engine + Dispatcher
           |
           v
Model Gateway (primary -> fallbacks)
           |
           v
Tools + Session Store + Audit + Metrics/Event Bus
```

## Configuration

Config path resolution order:

1. `--config`
2. `ACTIONAGENT_CONFIG`
3. `<binary-dir>/actionAgent.json`
4. system default path
- Linux/macOS: `/etc/actionagent/actionAgent.json`
- Windows: `C:\ProgramData\ActionAgent\acgtionAgent.json` (current implementation path)

Runtime behavior:

1. Exactly one resolved config file is loaded.
2. Field-level multi-source merge is not applied.
3. If the resolved file does not exist and the parent path is writable, defaults are auto-created.

Model provider recommendation (prefer env-based keys):

```json
{
  "model_gateway": {
    "primary": "openai-main",
    "fallbacks": ["anthropic-backup"],
    "providers": [
      {
        "name": "openai-main",
        "api_style": "openai",
        "base_url": "https://api.openai.com/v1",
        "api_key_env": "ACTIONAGENT_OPENAI_API_KEY",
        "model": "gpt-4o-mini",
        "timeout_ms": 20000,
        "max_attempts": 2,
        "enabled": true
      }
    ]
  }
}
```

## Deployment Helper Scripts

- Start agent (PowerShell): `./scripts/start-agent.ps1`
- Start agent (Bash): `./scripts/start-agent.sh`
- Verify model provider (PowerShell): `./scripts/verify-model-provider.ps1 -BaseUrl http://127.0.0.1:8000`
- Verify model provider (Bash): `./scripts/verify-model-provider.sh http://127.0.0.1:8000`

## Development Workflow

Repository layout:

- `agent/`: Go runtime kernel (`actionagentd`)
- `docs/prd/`: product/technical planning documents
- `agent/docs/`: API, architecture, and current status docs
- `openspec/`: change proposals/spec/tasks
- `scripts/`: startup and local helper scripts

Build and test:

```bash
cd agent
go test ./...
```

Recommended flow:

1. Confirm product and technical intent in `docs/prd/`.
2. Create or update a change in OpenSpec (`/opsx:propose`).
3. Implement tasks using `/opsx:apply` and keep task checkboxes in sync.
4. Run tests (`go test ./...`) before review.
5. Archive completed changes with `/opsx:archive <change-name>`.

Contribution and quality policy:

1. Commit messages must be English-only (ASCII).
2. Enable local commit hook:

```powershell
powershell -ExecutionPolicy Bypass -File ./scripts/setup-hooks.ps1
```

3. Keep code changes scoped to the active OpenSpec tasks.

## Roadmap

Current MVP baseline:

1. Single-process runtime (`actionagentd`) with task engine and dispatcher.
2. OpenAI-compatible API surface plus typed frame bridge.
3. Baseline observability (`healthz/events/metrics/alerts`) and audit output.

Next phases:

1. Multi-node relay hardening and richer recovery snapshots.
2. Production-grade approval workflows and stronger persistence.
3. Web UI and team-governance capabilities.

## Docs

- Core API: `agent/docs/API.md`
- Current status: `agent/docs/CURRENT.md`
- Architecture: `agent/docs/ARCHITECTURE.md`
- Core PRD: `agent/docs/PRD.md`
- Product planning: `docs/prd/actionagent-design.md`
- Agent kernel product design: `docs/prd/agent-kernel-product-design.md`
- Agent kernel technical solution: `docs/prd/agent-kernel-technical-solution.md`
- Model provider configuration: `docs/prd/agent-model-provider-configuration.md`
