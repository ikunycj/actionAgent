# ActionAgent

一个部署优先（Deployment-first）的分布式 Agent 平台，聚焦任务可执行性、可观测性与可审计性。

English version: [README.md](README.md)

## 1. 项目宏观介绍

### 项目定位

ActionAgent 的目标是在低部署成本下，提供可快速启动、稳定运行、可持续观测的 Agent 核心运行时。

核心价值：
1. 从客户端或 Web 入口快速发起临时任务。
2. 在本机或远端节点持续运行长任务。
3. 以日志和审计记录保障结果可追溯。

### 宏观架构

ActionAgent 采用“控制面 + 执行面”双平面模型。

1. Core（执行面核心）
- 形态：Go 单二进制运行时（`actionagentd`）
- 平台：Windows / Linux / macOS
- 职责：任务执行、模型路由、工具运行、日志、事件与审计输出

2. Client（控制面）
- 形态：桌面端/移动端（分阶段）
- 职责：发起任务、查看状态、处理审批、接收回执

3. Cloud Relay（可选）
- 职责：跨网络节点接力与协同

4. Team Console（后续）
- 职责：组织治理、策略模板、审计中心、节点编排

### 当前 MVP 范围

1. 单进程运行时（`actionagentd`）
2. 健康检查接口（`GET /healthz`）
3. OpenAI 兼容接口（`POST /v1/chat/completions`）
4. 直接执行接口（`POST /v1/run`）
5. Typed frame 桥接接口（`POST /ws/frame`）
6. 基础事件流与指标输出

### 当前实现进展（2026-03-08）

内核已达到“可运行、可观测、可扩展”的初步形态，当前已落地：
1. 配置加载与来源解析（`--config` / 环境变量 / 二进制目录 / 系统默认）
2. 任务引擎（车道并发、状态迁移、幂等去重）
3. 调度与终态收敛链路（含聚合输出）
4. 模型网关主备路由（primary + fallback）
5. 工具运行时分级权限与审批令牌校验
6. 会话转录与记忆降级检索能力
7. 事件总线与指标接口输出

后续阶段重点：
1. 多节点接力稳定性与恢复快照增强
2. 生产级审批流与持久化治理
3. WebUI 与团队治理能力完善

### 路线概览

当前仓库仍处于 MVP 持续迭代阶段，但内核主链路已具备可用基线。

## 2. 项目使用方法

### 环境要求

1. Go 1.25+（推荐 Go 1.25.8）
2. Windows/Linux/macOS 命令行环境

### 本机快速启动

在仓库根目录执行：

1. 构建

```bash
cd agent
go build -o actionagentd ./cmd/actionagentd
```

2. 通过显式配置路径启动（推荐）

```bash
./actionagentd --config "$(pwd)/actionAgent.json"
```

3. 健康检查

```bash
curl http://127.0.0.1:8787/healthz
```

### API 调用示例

1. OpenAI 兼容调用

```bash
curl -X POST http://127.0.0.1:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

2. 直接任务调用

```bash
curl -X POST http://127.0.0.1:8787/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "input":{"text":"Summarize this paragraph in Chinese."}
  }'
```

### 配置规则

配置路径解析优先级：
1. `--config`
2. `ACTIONAGENT_CONFIG`
3. `二进制所在目录/actionAgent.json`
4. 系统默认路径（优先级低于二进制目录）
- Linux：`/etc/<appname>/actionAgent.json`
- Windows：`C:\ProgramData\<AppName>\acgtionAgent.json`

运行时行为：
1. 仅加载一个已解析的配置文件。
2. 不做字段级多源合并。
3. 当解析路径文件不存在且可写时，自动初始化默认配置。

### 部署辅助脚本

1. PowerShell：`./scripts/start-agent.ps1`
2. Bash：`./scripts/start-agent.sh`

## 3. 项目开发方法

### 仓库结构

1. `agent/`：Agent 内核运行时实现（Go）
2. `docs/`：产品/技术设计与参考文档
3. `openspec/`：变更提案、规格、设计、任务追踪
4. `scripts/`：本地开发与启动辅助脚本

### 构建与测试

在 `agent/` 目录执行：

```bash
go test ./...
```

### 推荐开发流程

1. 先阅读并确认 `docs/design/` 下的产品与技术约束。
2. 使用 OpenSpec 创建或更新变更（`/opsx:propose`）。
3. 使用 `/opsx:apply` 实施任务，并同步更新任务勾选状态。
4. 提交评审前执行测试（`go test ./...`）。
5. 变更完成后执行归档（`/opsx:archive <change-name>`）。

### 代码质量与提交流程

1. Commit message 必须为英文（ASCII）。
2. 启用本地提交钩子：

```powershell
powershell -ExecutionPolicy Bypass -File ./scripts/setup-hooks.ps1
```

3. 代码改动应与当前 OpenSpec 任务保持一致且范围最小化。

### 相关文档

1. 总体产品规划：`docs/actionagent-design.md`
2. Agent 内核产品设计：`docs/design/agent-kernel-product-design.md`
3. Agent 内核技术方案：`docs/design/agent-kernel-technical-solution.md`
