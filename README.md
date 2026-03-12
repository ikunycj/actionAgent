# ActionAgent

ActionAgent is a self-hosted Agent product focused on simple deployment and user-controlled runtime ownership.

The product direction is now:

- one user owns one Core by default
- the Core release package includes both the Go backend and the WebUI assets
- the bundled WebUI is an agent-scoped management console, not a separately deployed product
- the native client is the primary daily-use surface
- the Core still exposes HTTP, bridge, and observability APIs for clients and automation

Chinese product docs:

- `docs/PRD.md`
- `docs/agent/PRD.md`
- `docs/webui/PRD.md`
- `docs/app/PRD.md`

## Highlights

- Self-hosted single-user Core
- Single-package delivery target for backend plus WebUI
- Same-origin WebUI console in production
- Native-client-first product direction
- OpenAI-compatible `POST /v1/chat/completions` and `POST /v1/responses`
- Direct task API: `POST /v1/run`
- Bridge API: `POST /ws/frame`
- Runtime catalog: `GET /v1/runtime/agents`
- Runtime observability: `GET /healthz`, `GET /events`, `GET /metrics`, `GET /alerts`

## Quick Start

Current developer path:

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

Bundled WebUI in repo development:

```bash
cd web
npm run build
cd ../agent
go build -o actionagentd ./cmd/actionagentd
./actionagentd --config "$(pwd)/actionAgent.json"
```

With `web/dist` available, Core will also serve:

- `GET /`
- `GET /app/agents/<agent-id>/overview`

Release package build:

```powershell
.\scripts\build-core-package.ps1
```

## Current Status

What exists now:

- Core runtime and APIs are implemented
- task execution, model routing, events, metrics, alerts, and session maintenance are available
- the `web/` application can now be served by Core on the same origin when built assets are present

What is changing at the product level:

- production delivery should move from "standalone WebUI + standalone Core" to "Core package with bundled WebUI"
- the WebUI should become a per-agent management console
- the native client should become the main user-facing interaction surface

What is still missing to match that direction:

- the WebUI still has placeholder pages for history, config editing, and diagnostics
- auth and config control APIs are still incomplete
- the native client is still only a planned product surface

## API Surface

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/healthz` | `GET` | liveness/readiness |
| `/v1/run` | `POST` | generic task execution |
| `/v1/runtime/agents` | `GET` | active agent catalog for bundled WebUI |
| `/v1/chat/completions` | `POST` | OpenAI Chat Completions compatible |
| `/v1/responses` | `POST` | OpenAI Responses style API |
| `/ws/frame` | `POST` | typed bridge request/response endpoint |
| `/events` | `GET` | realtime event stream |
| `/metrics` | `GET` | runtime metrics snapshot |
| `/alerts` | `GET` | alert evaluation output |

## Product Shape

```text
Native Client
   |
   v
Agent Core
   | \
   |  \__ Bundled WebUI console
   |
   +__ Model gateway
   +__ Task runtime
   +__ Sessions / memory / tools
   +__ Events / metrics / alerts
```

## Configuration

Config path resolution order:

1. `--config`
2. `ACTIONAGENT_CONFIG`
3. `<binary-dir>/actionAgent.json`
4. system default path

Runtime behavior:

1. Exactly one resolved config file is loaded.
2. Users configure `port`, and the service listens on `127.0.0.1:<port>` by default.
3. `--addr` remains an advanced override for full bind address.
4. Core looks for bundled WebUI assets in `<binary-dir>/webui` first, then falls back to repo `web/dist` during development.
5. Production delivery keeps WebUI and Core on the same origin.

## Repository Layout

- `agent/`: Go runtime module
- `web/`: bundled WebUI source
- `docs/`: product and module docs
- `openspec/`: change proposals, specs, and tasks
- `scripts/`: helper scripts

## Roadmap

Near-term:

1. Bundle WebUI into the Core release package.
2. Turn WebUI into an agent-scoped management console.
3. Promote the native client to the main daily-use product.
4. Complete auth, config control, history, and release packaging.

Longer-term:

1. Stronger persistence and governance.
2. Better multi-node and recovery behavior.
3. Unified management of remote Core and local Core from the client.
