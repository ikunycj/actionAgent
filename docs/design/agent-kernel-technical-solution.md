# ActionAgent Agent 内核技术方案（TS）

## 0. 文档信息
- 文档名称：Agent 内核技术方案
- 适用范围：`actionAgent/agent` 技术实现蓝图（本稿不含代码）
- 版本：v1.0
- 日期：2026-03-07
- 关联文档：
  - `actionAgent/docs/actionagent-design.md`
  - `actionAgent/docs/design/agent-kernel-product-design.md`

## 1. 方案目标
1. 将 PRD 中的内核能力落为可实施的工程架构。
2. 在 MVP 阶段优先保障稳定完成任务与故障恢复。
3. 提供可扩展边界，支持后续多节点、MCP、Skills、团队化治理。

## 2. 借鉴映射（OpenClaw -> ActionAgent）

### 2.1 运行时与协议
1. 借鉴 typed WS 协议帧设计（req/res/event + schema 校验）。
2. 借鉴首帧握手与设备身份绑定机制。
3. 借鉴控制面与执行面复用同一网关协议的做法。

### 2.2 队列与幂等
1. 借鉴 lane-aware queue（按 lane 并发、按 session 串行）。
2. 借鉴幂等去重缓存 + 定时清理（TTL + 上限）。
3. 借鉴运行终态双通道判定（生命周期事件 + 去重缓存）。

### 2.3 安全与审批
1. 借鉴高风险执行审批对象（allow-once / allow-always）。
2. 借鉴审批绑定（设备/节点/运行上下文）与一次性消费机制。
3. 借鉴高危工具默认 deny 与安全审计策略。

### 2.4 会话与记忆
1. 借鉴会话 key 规范化与 DM 隔离策略。
2. 借鉴会话维护策略（prune/cap/rotate/disk budget）。
3. 借鉴记忆降级策略（embedding 不可用时 FTS-only）。

### 2.5 模型治理
1. 借鉴 failover 错误分类。
2. 借鉴 auth profile 轮转与 cooldown/billing-disable 策略。
3. 借鉴模型 fallback 链和会话粘性。

## 3. 总体架构

### 3.1 单二进制内核拓扑
一个进程内包含：
1. API Gateway（HTTP/WS，对外协议与控制面入口）。
2. Task Engine（任务状态机 + 队列 + 幂等）。
3. Dispatcher（本地/远端调度与接力）。
4. Model Gateway（多供应商适配、路由、预算、fallback）。
5. Tool Runtime（工具注册、权限、审批、执行审计）。
6. Session & Memory（会话、转录、记忆索引、维护）。
7. Event Bus（事件流、指标、日志、追踪）。

### 3.2 角色模式
同一二进制通过启动参数切换角色：
1. `controller`
2. `worker`
3. `hybrid`（单机默认）
4. `edge-worker`

## 4. 模块规划（`actionAgent/agent`）

建议模块边界：
1. `kernel/gateway`：协议、鉴权、请求路由、WS 连接管理。
2. `kernel/task`：任务实体、状态机、幂等、队列、重试。
3. `kernel/dispatch`：节点能力表、调度器、接力策略。
4. `kernel/model`：供应商适配器、路由策略、故障分类、fallback。
5. `kernel/tools`：工具目录、策略管线、审批集成、执行代理。
6. `kernel/session`：会话 key、会话存储、转录、维护任务。
7. `kernel/memory`：记忆文件层、索引层、查询层、降级策略。
8. `kernel/events`：事件模型、发布订阅、持久化输出。
9. `kernel/security`：密钥引用、审计、策略校验、风险扫描。
10. `kernel/config`：配置加载、diff、热重载计划。
11. `kernel/storage`：KV/SQLite/对象存储抽象。
12. `kernel/observability`：metrics、trace、structured log。

## 5. 核心数据模型（逻辑）

### 5.1 任务模型
1. `Task`：`task_id`、`idempotency_key`、`status`、`priority`、`timeout_ms`。
2. `TaskRun`：`run_id`、`task_id`、`lane`、`attempt`、`node_id`、`started_at`、`ended_at`。
3. `TaskSnapshot`：接力用上下文快照（可序列化）。

### 5.2 审批模型
1. `ApprovalRequest`：命令摘要、风险等级、发起方、目标节点、过期时间。
2. `ApprovalDecision`：`allow-once` / `allow-always` / `deny` / `timeout`。
3. `ApprovalBinding`：绑定设备 ID、连接 ID、run_id、node_id。

### 5.3 会话模型
1. `SessionEntry`：`session_key` -> `session_id`、`updated_at`、策略覆盖。
2. `SessionTranscript`：按会话记录事件/对话 JSONL。
3. `SessionMaintenancePolicy`：`warn|enforce`、`prune_after`、`max_entries`、`max_disk`。

### 5.4 去重模型
1. `DedupeEntry`：`key`、`ts`、`ok`、`payload|error`。
2. 生命周期：写入、查询、定时过期清理、超量淘汰。

## 6. 状态机与流程

### 6.1 任务状态迁移
1. `CREATED -> QUEUED -> DISPATCHING -> RUNNING`。
2. 风险工具触发 `RUNNING -> WAITING_APPROVAL -> RUNNING`。
3. 异常路径 `RUNNING -> RETRYING -> RUNNING | FAILED`。
4. 用户取消 `* -> CANCELLED`。
5. 终态只允许 `SUCCEEDED|FAILED|CANCELLED` 三种。

### 6.2 终态判定策略
1. 主判定来源：生命周期流（`start/end/error`）。
2. 补判定来源：幂等去重缓存终态。
3. 错误缓冲窗口：短时间内等待可能的 fallback 成功事件。
4. 以最新时间戳终态为准，避免旧 run 污染。

### 6.3 任务接力（Relay）
1. 识别不可用节点（心跳、调用失败、策略不满足）。
2. 生成 `TaskSnapshot` 并转交候选 Worker。
3. 新节点续跑并继承同一 `task_id` 语义。
4. Controller 聚合多次 run，输出单一终态回执。

## 7. 协议与接口方案

### 7.1 网关帧协议（WS）
1. 请求帧：`{type:"req", id, method, params}`。
2. 响应帧：`{type:"res", id, ok, payload|error}`。
3. 事件帧：`{type:"event", event, payload, seq}`。
4. 首帧必须 `connect`，携带客户端/设备/角色信息。

### 7.2 对外 HTTP 兼容层
1. `/v1/chat/completions`（OpenAI 风格）。
2. `/v1/responses`（统一响应风格，可承载工具调用）。
3. Anthropic 兼容入口（请求/响应字段映射）。
4. 兼容层只做协议转换，不承担业务调度逻辑。

### 7.3 控制面方法（最小集）
1. `agent.run`：提交任务。
2. `agent.wait`：等待终态。
3. `task.get` / `task.list`：查询。
4. `node.list` / `node.describe` / `node.invoke`。
5. `exec.approval.request` / `exec.approval.resolve`。
6. `audit.query` / `audit.export`。

## 8. 调度与并发方案

### 8.1 车道并发
1. 车道级并发控制（`main/cron/subagent/session:*`）。
2. `session:*` 默认串行，避免同会话并发写冲突。
3. 支持动态调整 `maxConcurrent`。

### 8.2 背压与排队策略
1. 入队等待超阈值告警。
2. 队列模式支持 `steer/followup/collect/steer-backlog/interrupt`。
3. 队列默认参数：`debounce=1000ms`、`cap=20`、`drop=summarize`。

### 8.3 关停策略
1. 标记 draining 后拒绝新任务。
2. 等待活跃任务收敛到超时阈值。
3. 未完成任务写入恢复点。

## 9. 模型网关方案

### 9.1 路由策略
1. `primary -> fallbacks[]` 顺序尝试。
2. 单次请求仅重试当前步骤，避免多步重复副作用。
3. 会话维持选中凭据粘性，减少抖动。

### 9.2 凭据与冷却
1. 失败类型驱动 cooldown。
2. rate-limit/timeout 进入短冷却（指数退避）。
3. billing 进入长禁用窗口。
4. 过期冷却自动清理并重置错误计数。

## 10. 工具运行时方案

### 10.1 工具策略管线
1. 全局策略。
2. 供应商策略。
3. Agent 级策略。
4. 会话级覆盖策略。
5. 插件工具分组扩展策略。

### 10.2 风险拦截
1. 高风险工具默认 deny。
2. 非 owner 调用 owner-only 工具直接拒绝。
3. `allow-once` 审批执行后立即消费。
4. 审批绑定 mismatch（设备/节点/参数）一律拒绝。

## 11. 会话与记忆方案

### 11.1 会话键策略
1. 默认主会话：`agent:<agent_id>:main`。
2. 支持 DM 隔离模式：`main/per-peer/per-channel-peer/per-account-channel-peer`。
3. 群组会话天然隔离，thread 追加后缀。

### 11.2 会话维护
1. 写路径触发轻量维护。
2. 支持手动 cleanup。
3. enforce 模式下执行清理/轮转/磁盘预算控制。

### 11.3 记忆检索
1. Markdown 文件为真源。
2. embedding 可用时：hybrid（vector + FTS）。
3. embedding 不可用时：FTS-only 退化。
4. 不存在文件读取返回空文本，不抛错。

## 12. 安全与治理方案

### 12.1 认证鉴权
1. 网关 token + 设备身份校验。
2. 控制面能力按 scope 授权。
3. 控制面写操作限流（按设备 + IP）。

### 12.2 审计
1. 工具调用全量审计。
2. 审批决策全量审计。
3. 配置变更与热重载计划写审计。
4. 审计支持脱敏导出。

### 12.3 安全基线扫描
1. 高危配置检测。
2. 权限面暴露检测。
3. 插件代码风险摘要。
4. 文件权限与密钥泄漏风险检测。

## 13. 配置与热重载方案
1. 配置变更先做 `diff paths`。
2. 生成 `reload plan`：`hot/restart/noop`。
3. 可热重载项直接生效（如 heartbeat、hooks、策略参数）。
4. 必须重启项排队进入安全重启流程。

## 14. 可观测性方案

### 14.1 事件
1. 每个 `run_id` 生成严格单调 `seq`。
2. 事件域覆盖 lifecycle/tool/assistant/error/system。
3. WS 实时推送 + 持久化留痕双写。

### 14.2 指标（最小集）
1. 任务成功率、失败率、平均耗时、P95。
2. 队列深度、等待时长、活跃并发。
3. 节点在线率、心跳丢失率、接力成功率。
4. 审批通过率、审批超时率。

## 15. 测试与验收策略

### 15.1 测试分层
1. 单元测试：状态机、路由策略、工具策略。
2. 组件测试：队列、去重、审批流、会话维护。
3. 集成测试：Controller/Worker/Edge 端到端。
4. 稳定性测试：长任务、网络抖动、节点切换。

### 15.2 关键回归用例
1. 同 idempotency_key 重放只返回一个终态。
2. fallback 期间错误不应提前终态。
3. 审批 once 不可重放。
4. 节点断连后任务可接力。
5. 配置热重载不会破坏在途任务。

## 16. 里程碑与交付物
1. M1：内核骨架 + 状态机 + 车道队列 + 基础协议。
2. M2：模型网关 fallback + 工具审批 + 审计链路。
3. M3：分布式接力 + 会话维护 + 热重载 + 稳定性验收。

每个里程碑交付：
1. 架构说明更新。
2. 接口契约文档更新。
3. 验收报告（KPI 对齐）。

---
本技术方案用于指导 `actionAgent/agent` 实现阶段，强调“先稳定、再扩展、全程可审计”。
