# ActionAgent (deployment-first MVP)

A single-binary Go agent runtime focused on low deployment cost.

## What this MVP does

- Runs as one process (`actionagentd`) on Windows/Linux/macOS.
- Exposes OpenAI-compatible endpoint: `POST /v1/chat/completions`.
- Supports direct prompt execution: `POST /v1/run`.
- Supports optional local scheduled jobs.
- Uses environment variables by default; `config.json` is optional.

## Quick start

1. Build:

```bash
go build -o actionagentd ./cmd/actionagentd
```

2. Run (with env only):

```bash
ACTIONAGENT_API_KEY=sk-xxx ./actionagentd
```

3. Health check:

```bash
curl http://127.0.0.1:8787/healthz
```

4. Test chat completion:

```bash
curl -X POST http://127.0.0.1:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

## Config

Optional file: `config.json` (see `config.example.json`).

Environment overrides:

- `ACTIONAGENT_ADDR` (default `127.0.0.1:8787`)
- `ACTIONAGENT_UPSTREAM_BASE_URL` (default `https://api.openai.com/v1`)
- `ACTIONAGENT_API_KEY`
- `ACTIONAGENT_DEFAULT_MODEL` (default `gpt-4o-mini`)
- `ACTIONAGENT_REQUEST_TIMEOUT_SECONDS` (default `120`)
- `ACTIONAGENT_SYSTEM_PROMPT`

Legacy `GOCLAW_*` variables are still supported for backward compatibility.

## Deployment-first notes

- No Node/npm dependency chain.
- No mandatory database.
- Can be wrapped by systemd/launchd/Windows Service later.
- Mobile should use this runtime as execution plane (desktop/cloud), while phone acts as control plane.



