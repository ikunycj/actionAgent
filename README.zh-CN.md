# ActionAgent

部署优先（Deployment-first）的分布式通用 Agent 平台。

## 产品定位

ActionAgent 致力于以最低部署成本提供可执行、可协作、可扩展的 Agent 能力。

核心价值：
1. 客户端或网页端发起临时任务。
2. 本机/远端节点持续执行长任务。
3. 结果可回推、可审计、可追溯。

English version: [README.md](README.md)

## 产品形态概述

ActionAgent 采用“控制面 + 执行面”双平面架构。

### 1) Core（执行面核心）
- 形态：Go 单二进制运行时（`actionagentd`）
- 部署：Windows / Linux / macOS
- 作用：任务执行、模型调用、工具运行、日志与审计输出
- 当前状态：MVP 已支持（本机单节点）

### 2) Client（控制面）
- 形态：桌面端 / 移动端（分阶段）
- 作用：发起任务、查看状态、处理审批、接收回执
- 当前状态：当前主要通过 HTTP API 作为控制入口

### 3) Cloud Relay（可选）
- 作用：跨网络节点协同与任务接力
- 当前状态：规划中

### 4) Team Console（后续）
- 作用：组织权限、策略模板、审计中心、节点编排
- 当前状态：规划中

## 当前 MVP 能力
1. 单进程运行 `actionagentd`
2. 健康检查 `GET /healthz`
3. OpenAI 兼容接口 `POST /v1/chat/completions`
4. 直接执行接口 `POST /v1/run`
5. 环境变量优先配置，`config.json` 可选

## 产品使用方法

### A. 本机快速启动（推荐）

1. 构建二进制

```bash
go build -o actionagentd ./cmd/actionagentd
```

2. 设置最小运行变量并启动

```bash
ACTIONAGENT_API_KEY=sk-xxx ./actionagentd
```

3. 健康检查

```bash
curl http://127.0.0.1:8787/healthz
```

### B. OpenAI 兼容模式调用

```bash
curl -X POST http://127.0.0.1:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

### C. 直接任务执行调用

```bash
curl -X POST http://127.0.0.1:8787/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "input":"Summarize this paragraph in Chinese.",
    "model":"gpt-4o-mini"
  }'
```

说明：`/v1/run` 的请求字段以服务端当前实现为准。

## 典型场景
1. 临时任务：一句话触发，快速返回结果
2. 长任务：通过后端或定时触发，后台执行并回执
3. 跨设备协作（规划中）：手机触发、桌面/云端执行

## 配置说明

可选配置文件：`config.json`（参考 `config.example.json`）。

环境变量（优先级高于配置文件）：
- `ACTIONAGENT_ADDR`（默认 `127.0.0.1:8787`）
- `ACTIONAGENT_UPSTREAM_BASE_URL`（默认 `https://api.openai.com/v1`）
- `ACTIONAGENT_API_KEY`
- `ACTIONAGENT_DEFAULT_MODEL`（默认 `gpt-4o-mini`）
- `ACTIONAGENT_REQUEST_TIMEOUT_SECONDS`（默认 `120`）
- `ACTIONAGENT_SYSTEM_PROMPT`

兼容变量：
- 仍支持 `GOCLAW_*` 变量。

## Commit Message 规范

提交信息必须使用英文（ASCII 字符集）。

仓库通过以下方式强制：
1. 本地 Git Hook：`.githooks/commit-msg`
2. CI 工作流：`.github/workflows/commit-message-english.yml`

启用本地 Hook 路径：

```powershell
powershell -ExecutionPolicy Bypass -File ./scripts/setup-hooks.ps1
```

## 部署建议
1. 开发环境：使用环境变量快速验证闭环
2. 生产环境：固定配置文件 + 进程守护（systemd / launchd / Windows Service）
3. 安全建议：API Key 使用安全存储，不要提交到仓库

## 文档导航
- 总体产品规划：`docs/actionagent-design.md`
- Agent 内核产品设计：`docs/design/agent-kernel-product-design.md`
- Agent 内核技术方案：`docs/design/agent-kernel-technical-solution.md`

## 路线说明
当前仓库处于 MVP 演进阶段，分布式接力、审批流、团队治理等能力将按路线图分阶段上线。
