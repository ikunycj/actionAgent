# ActionAgent - 自托管 Agent Core

ActionAgent 是一个面向自部署用户的通用 Agent 产品，目标是让用户在自己的基础设施上部署并控制自己的 Agent Core。

当前产品方向：

- 默认一个用户拥有一个 Agent Core
- Core 是执行面和事实源
- Core 只提供 HTTP、WS、事件流等 API 能力
- WebUI 是独立产品，通过 API 连接 Core
- 后续 Windows、macOS、Android 原生客户端既可以远程控制云端 Core，也可以管理本地独立 Core

英文版： [README.md](README.md)

[快速开始](#快速开始) | [当前状态](#当前状态) | [API 总览](#api-总览) | [配置规则](#配置规则) | [路线图](#路线图)

## 核心亮点

- 单用户、自托管 Core，适合部署在自己的云服务器或本机环境。
- 单二进制运行时目标，支持 Windows/Linux/macOS。
- API-only Core：不内嵌 WebUI 静态资源。
- OpenAI 兼容接口：`POST /v1/chat/completions` 和 `POST /v1/responses`。
- 通用任务接口：`POST /v1/run`，支持 lane、session、幂等键。
- typed bridge 接口：`POST /ws/frame`，当前支持 `task.get`、`task.list`、`agent.wait`、`session.stats`、`session.maintain`。
- 可观测接口：`GET /healthz`、`GET /events`、`GET /metrics`、`GET /alerts`。
- 配置文件仍是模型厂商、API 风格、运行参数的最终事实源。

## 快速开始

发布态目标路径：

```bash
./actionagentd --config ./actionAgent.json
```

当前仓库开发路径：

```bash
cd agent
go build -o actionagentd ./cmd/actionagentd
./actionagentd --config "$(pwd)/actionAgent.json"
```

PowerShell：

```powershell
cd agent
go build -o actionagentd.exe ./cmd/actionagentd
.\actionagentd.exe --config "$PWD\actionAgent.json"
```

健康检查：

```bash
curl http://127.0.0.1:8000/healthz
```

注意：

- 当前二进制只启动 Core
- WebUI 不会被打包进 Core 二进制
- WebUI 需要单独构建、单独部署，再连接 Core 的 Base URL

## 当前状态

代码库中已经存在的能力：

- Core runtime 和 HTTP/WS API 已实现。
- 任务执行、任务查询、事件流、指标、告警、会话维护已可用。
- `openai` 和 `anthropic` provider 路由已实现。

首发产品还缺少的能力：

- 独立 WebUI 交付
- 认证接口
- 配置读取、修改、应用接口
- 完整对话历史读取接口
- 面向用户的二进制发布包装

当前已知限制：

- `task.list` 已经可通过 `/ws/frame` 使用。
- 完整对话历史目前还没有公开 API。
- 当前内部 transcript 仅在 `run` 路径写入；`chat.completions` 和 `responses` 还没有形成 WebUI 可直接使用的历史读取闭环。
- Core 当前不提供 WebUI 静态资源托管。

## API 总览

| 接口 | 方法 | 用途 |
| --- | --- | --- |
| `/healthz` | `GET` | 存活/就绪检查 |
| `/v1/run` | `POST` | 通用任务执行 |
| `/v1/chat/completions` | `POST` | OpenAI Chat Completions 兼容接口 |
| `/v1/responses` | `POST` | OpenAI Responses 风格接口 |
| `/ws/frame` | `POST` | typed bridge 请求/响应接口 |
| `/events` | `GET` | 实时事件流（JSON lines） |
| `/metrics` | `GET` | 运行指标快照 |
| `/alerts` | `GET` | 告警评估结果 |

当前已实现的 bridge 方法：

- `agent.run`
- `agent.wait`
- `task.get`
- `task.list`
- `audit.query`
- `approval.list`
- `session.stats`
- `session.maintain`
- `observability.alerts`

Chat Completions 示例：

```bash
curl -X POST http://127.0.0.1:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "session_key":"chat-main",
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

通用任务示例：

```bash
curl -X POST http://127.0.0.1:8000/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id":"default",
    "session_key":"chat-main",
    "input":{"text":"Summarize this paragraph in Chinese."}
  }'
```

任务列表示例：

```bash
curl -X POST http://127.0.0.1:8000/ws/frame \
  -H "Content-Type: application/json" \
  -d '{
    "type":"req",
    "id":"list-1",
    "method":"task.list",
    "params":{"limit":10}
  }'
```

## 工作方式

```text
独立 WebUI / 后续原生客户端
              |
              v
            API 调用
              |
              v
          Agent Core
 (gateway + task engine + model runtime)
              |
              v
          Model Gateway
      (primary -> fallbacks)
              |
              v
   Tools + Sessions + Audit + Metrics
```

## 配置规则

配置路径解析顺序：

1. `--config`
2. `ACTIONAGENT_CONFIG`
3. `<binary-dir>/actionAgent.json`
4. 系统默认路径

系统默认路径：

- Linux/macOS：`/etc/actionagent/actionAgent.json`
- Windows：当前实现为 `C:\ProgramData\ActionAgent\acgtionAgent.json`

运行时行为：

1. 只加载一个最终解析出的配置文件。
2. 不做字段级多源合并。
3. 若目标配置文件不存在且父目录可写，会自动生成默认配置。
4. 用户可以通过配置切换 provider 厂商、`api_style`、model、timeout、retry 等运行参数。

模型 Provider 配置示例：

```json
{
  "port": 8000,
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

- 启动 Agent（PowerShell）：`./scripts/start-agent.ps1`
- 启动 Agent（Bash）：`./scripts/start-agent.sh`
- Provider 验证（PowerShell）：`./scripts/verify-model-provider.ps1 -BaseUrl http://127.0.0.1:8000`
- Provider 验证（Bash）：`./scripts/verify-model-provider.sh http://127.0.0.1:8000`

## 开发流程

仓库结构：

- `agent/`：Go Core runtime 模块（`actionagentd`）
- `agent/internal/app/runtime/`：运行时编排、启动、model runtime、agent registry
- `agent/internal/adapter/httpapi/`：HTTP 与 typed bridge 入口
- `agent/internal/core/`：task、dispatch、model、tools、session、memory 能力
- `agent/internal/platform/`：config、events、observability、storage 服务
- `web/`：独立 WebUI 源码
- `docs/`：产品和模块文档
- `docs/agent/`：Core API、状态、PRD 文档
- `docs/webui/`：WebUI 产品文档
- `docs/app/`：客户端产品文档
- `openspec/`：变更提案、规格、任务
- `scripts/`：启动和本地辅助脚本

构建与测试：

```bash
cd agent
go test ./...
```

推荐流程：

1. 先确认 `docs/PRD.md` 和 `docs/` 下各模块文档。
2. 使用 OpenSpec 创建或更新变更。
3. 按任务实施，并保持任务状态同步。
4. 评审前执行测试。
5. 完成后归档变更。

## 路线图

近期里程碑：

1. 认证与配置控制面。
2. 独立 WebUI，对话、完整历史、任务列表闭环。
3. Core 二进制优先发布包装。
4. Windows、macOS、Android 原生客户端。

后续方向：

1. 多节点 relay 加固与恢复快照增强。
2. 更强的持久化和治理能力。
3. 远程 Core 与本地独立 Core 的统一管理。

## 文档索引

- 产品 PRD：`docs/PRD.md`
- Core API：`docs/agent/API.md`
- Core 状态：`docs/agent/CURRENT.md`
- Core PRD：`docs/agent/PRD.md`
- WebUI PRD：`docs/webui/PRD.md`
- Native Client PRD：`docs/app/PRD.md`
