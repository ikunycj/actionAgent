# ActionAgent Core Agent PRD

## 1. 文档信息
- 模块：`actionAgent/agent` 核心内核
- 版本：v1.0
- 日期：2026-03-08
- 依据文档：
  - `docs/prd/agent-kernel-product-design.md`
  - `docs/prd/agent-kernel-technical-solution.md`
  - 当前代码实现（`agent/kernel/*`）

## 2. 产品目标
1. 单二进制快速可用：启动后可完成健康检查与首个任务闭环。
2. 任务执行稳态可控：具备队列、状态机、重试、幂等去重与终态聚合。
3. 模型调用可治理：支持多 provider、primary/fallback、错误分类、重试退避。
4. 默认可观测与可审计：事件、指标、告警、工具执行审计可查询。
5. 面向多 Agent 配置：同一实例支持多个 `agent_id` 及默认路由。

## 3. 目标用户
1. 开发者：希望 5 分钟内本地启动并调试任务执行链路。
2. 进阶使用者：希望任务可追踪、失败可回放、调用可审计。
3. 管理/运维角色：希望通过指标和告警快速识别运行风险。

## 4. 本期范围（MVP）
1. 网关接口：`/healthz`、`/v1/run`、`/v1/chat/completions`、`/v1/responses`、`/ws/frame`、`/events`、`/metrics`、`/alerts`。
2. Task Engine：lane 并发、状态迁移合法性、幂等去重、draining。
3. Dispatcher：local-first 选路、节点健康衰减、快照接力骨架。
4. Model Gateway：OpenAI/Anthropic 风格适配、primary/fallback、凭据冷却与熔断。
5. Tool Runtime：L0/L1/L2 分级、审批 token、审计落盘。
6. Session/Memory：会话键规范化、转录维护策略、向量不可用时 FTS 降级。
7. 配置治理：固定优先级解析、单配置源加载、热更新分类（hot/restart/noop）。

## 5. 非目标（本期不承诺）
1. 完整生产级多节点编排与自治恢复。
2. 完整 WebUI 控制台与团队治理中心。
3. 全量外部存储持久化（当前多数运行数据仍为内存态）。
4. 完整的对外认证授权体系（当前网关未内建 token 鉴权流）。

## 6. 核心需求

### 6.1 配置与启动
1. 支持配置路径解析顺序：`--config` > `ACTIONAGENT_CONFIG` > `<binary-dir>/actionAgent.json` > 系统默认路径。
2. 运行时仅加载一个配置源，不做多源字段级 merge。
3. 配置缺失时自动生成默认配置文件（若路径可写）。
4. 保证 `default_agent` 与 `agents[]` 可校验且可回退兼容旧单 agent 配置。

### 6.2 任务执行
1. 提供状态机：`CREATED` 到终态（`SUCCEEDED|FAILED|CANCELLED`）。
2. 支持 lane 并发控制，`session:*` lane 默认串行。
3. 支持 `idempotency_key` 去重与 replay 返回。
4. 支持运行中 draining：拒绝新任务，等待已有任务收敛。

### 6.3 调度与接力
1. 默认 local-first 调度，能力匹配失败再尝试远端节点。
2. 节点维护心跳与过期标记，在线节点数可观测。
3. 保留任务快照与 relay 接口，为多节点增强阶段提供基础。

### 6.4 模型网关
1. provider 需支持 `api_style=openai|anthropic`。
2. 执行策略为 `primary -> fallbacks`。
3. 必须区分错误类别（rate_limit、timeout、auth、billing、format、model_not_found、unknown）。
4. 对可重试错误执行退避重试；对永久错误直接失败。
5. 支持流式响应透传（OpenAI `/responses` stream）。

### 6.5 工具与审批
1. L2 工具执行必须持有并通过审批 token 校验。
2. 支持 `allow-once`、`allow-always`，once 模式消费后失效。
3. 审批绑定信息需校验（设备/节点/run/scope）。
4. 所有工具执行写审计记录并支持查询过滤。

### 6.6 可观测性
1. 暴露任务、队列、节点、审批等指标快照。
2. 提供告警计算接口（queue depth、approval timeout、node online、failure rate）。
3. 事件总线需按 `run_id` 单调递增 `seq` 并支持实时订阅。

## 7. 验收标准
1. 可启动：`actionagentd` 启动后 `GET /healthz` 返回可用状态。
2. 可执行：`/v1/run`、`/v1/chat/completions`、`/v1/responses` 能返回终态。
3. 可追踪：`/events` 输出事件帧，`/metrics` 可见关键指标。
4. 可审计：工具审批 token 与审计记录可查询。
5. 可配置：多 agent 路由可通过 `agent_id` 或 `X-Agent-ID` 生效。

## 8. 里程碑建议
1. M1（已完成骨架）：内核启动、任务链路、基础接口。
2. M2（进行中）：模型路由增强、审批持久化、告警完善。
3. M3（规划中）：多节点接力稳定化、WebUI 联调、治理闭环。
