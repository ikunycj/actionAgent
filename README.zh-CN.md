# ActionAgent - 部署优先的 Agent 运行时

一个部署优先（Deployment-first）的分布式 Agent 运行时，聚焦任务可执行性、可观测性与可审计性。

English version: [README.md](README.md)

[快速开始](#快速开始tldr) | [API 总览](#api-总览) | [配置规则](#配置规则) | [开发流程](#开发流程) | [路线图](#路线图)

ActionAgent 采用“控制面 + 执行面”双平面：客户端或 Web 入口触发任务，`actionagentd` 负责执行、回传，并输出可追踪的日志、指标、事件和审计记录。

## 核心能力亮点

- 单二进制运行时（`actionagentd`），支持 Windows/Linux/macOS。
- OpenAI 兼容接口：`POST /v1/chat/completions` 与 `POST /v1/responses`。
- 任务执行接口：`POST /v1/run`，支持车道/会话/幂等键。
- Typed frame 桥接接口：`POST /ws/frame`，可做 req/res/event 集成。
- 可观测接口：`GET /healthz`、`GET /events`、`GET /metrics`、`GET /alerts`。
- 多 Agent 选择优先级固定：`body.agent_id` > `X-Agent-ID` > `default_agent`。
- 模型网关支持 `primary + fallbacks`，Provider 适配 `openai` 与 `anthropic`。

## 快速开始（TL;DR）

运行环境：Go `1.25+`（仓库 toolchain：`go1.25.8`）。

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

健康检查：

```bash
curl http://127.0.0.1:8000/healthz
```

## API 总览

| 接口 | 方法 | 用途 |
| --- | --- | --- |
| `/healthz` | `GET` | 存活/就绪检查 |
| `/v1/run` | `POST` | 通用任务执行 |
| `/v1/chat/completions` | `POST` | OpenAI Chat Completions 兼容入口 |
| `/v1/responses` | `POST` | OpenAI Responses 风格入口（支持流式透传） |
| `/ws/frame` | `POST` | typed frame 请求/响应桥接 |
| `/events` | `GET` | 实时事件流（JSON 行流，非 SSE） |
| `/metrics` | `GET` | 运行指标快照 |
| `/alerts` | `GET` | 告警评估结果 |

OpenAI 兼容调用示例：

```bash
curl -X POST http://127.0.0.1:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

直接任务调用示例：

```bash
curl -X POST http://127.0.0.1:8000/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "input":{"text":"Summarize this paragraph in Chinese."}
  }'
```

## 工作方式（简版）

```text
客户端 / CLI / 后续 UI
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

## 配置规则

配置路径解析顺序：

1. `--config`
2. `ACTIONAGENT_CONFIG`
3. `<二进制目录>/actionAgent.json`
4. 系统默认路径
- Linux/macOS：`/etc/actionagent/actionAgent.json`
- Windows：`C:\ProgramData\ActionAgent\acgtionAgent.json`（当前实现路径）

运行时行为：

1. 只加载一个最终解析出的配置文件。
2. 不做字段级多来源合并。
3. 若解析路径不存在且父目录可写，会自动生成默认配置。

模型 Provider 配置建议（优先使用环境变量注入密钥）：

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

## 部署辅助脚本

- 启动（PowerShell）：`./scripts/start-agent.ps1`
- 启动（Bash）：`./scripts/start-agent.sh`
- Provider 验证（PowerShell）：`./scripts/verify-model-provider.ps1 -BaseUrl http://127.0.0.1:8000`
- Provider 验证（Bash）：`./scripts/verify-model-provider.sh http://127.0.0.1:8000`

## 开发流程

仓库结构：

- `agent/`：Go 内核运行时（`actionagentd`）
- `docs/prd/`：产品/技术规划文档
- `agent/docs/`：接口、架构与当前状态文档
- `openspec/`：变更提案、规格、任务追踪
- `scripts/`：启动与本地辅助脚本

构建与测试：

```bash
cd agent
go test ./...
```

推荐流程：

1. 先确认 `docs/prd/` 下的产品与技术约束。
2. 使用 OpenSpec 创建或更新变更（`/opsx:propose`）。
3. 使用 `/opsx:apply` 实施任务，并同步更新任务勾选状态。
4. 提交评审前执行测试（`go test ./...`）。
5. 变更完成后归档（`/opsx:archive <change-name>`）。

代码质量与提交流程：

1. Commit message 必须为英文（ASCII）。
2. 启用本地提交钩子：

```powershell
powershell -ExecutionPolicy Bypass -File ./scripts/setup-hooks.ps1
```

3. 代码改动应与当前 OpenSpec 任务保持一致，范围尽量最小。

## 路线图

当前 MVP 基线：

1. 单进程运行时（`actionagentd`）+ 任务引擎 + 调度器。
2. OpenAI 风格接口与 typed frame 桥接接口可用。
3. 可观测链路（`healthz/events/metrics/alerts`）与审计输出已打通。

后续阶段重点：

1. 多节点接力稳定性与恢复快照增强。
2. 生产级审批流与持久化治理能力完善。
3. Web UI 与团队治理能力建设。

## 文档索引

- 核心 API：`agent/docs/API.md`
- 当前状态：`agent/docs/CURRENT.md`
- 架构文档：`agent/docs/ARCHITECTURE.md`
- 内核 PRD：`agent/docs/PRD.md`
- 产品规划：`docs/prd/actionagent-design.md`
- 内核产品设计：`docs/prd/agent-kernel-product-design.md`
- 内核技术方案：`docs/prd/agent-kernel-technical-solution.md`
- 模型 Provider 配置：`docs/prd/agent-model-provider-configuration.md`
