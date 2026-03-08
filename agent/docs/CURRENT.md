# ActionAgent Core Agent Current Status

## 1. 快照信息
- 日期：2026-03-08
- 范围：`actionAgent/agent` 核心模块
- 结论：已具备“可启动、可执行、可观测、可审计”的 MVP 核心闭环

## 2. 已完成能力
1. 启动与配置：
   - 单二进制 `actionagentd` 可启动。
   - 配置优先级解析与单配置源加载已实现。
   - 旧单 agent 配置可自动补齐 `default_agent`/`agents`。
2. 网关接口：
   - `GET /healthz`
   - `POST /v1/run`
   - `POST /v1/chat/completions`
   - `POST /v1/responses`（含 stream 透传）
   - `POST /ws/frame`
   - `GET /events`、`/metrics`、`/alerts`
3. 任务内核：
   - lane-aware queue（session lane 串行）。
   - 幂等去重（`idempotency_key`）与 replay 返回。
   - draining 控制与终态聚合。
4. 模型网关：
   - OpenAI/Anthropic 协议适配。
   - primary/fallback、错误分类、重试退避、熔断、凭据冷却。
5. 工具与治理：
   - 风险分级（L0/L1/L2）。
   - 审批 token（allow-once/allow-always）。
   - 工具审计与状态落盘（`state/tools-state.json`）。
6. 会话与记忆：
   - 会话 key 规范化与会话维护策略。
   - memory 的 vector -> FTS 降级能力。
7. 可观测：
   - 结构化日志、事件总线、指标聚合、阈值告警。

## 3. 当前架子（目录）
1. `agent/cmd/actionagentd/main.go`
2. `agent/kernel/runtime.go`
3. `agent/kernel/gateway/server.go`
4. `agent/kernel/task/task.go`
5. `agent/kernel/dispatch/dispatch.go`
6. `agent/kernel/model/*`
7. `agent/kernel/tools/tools.go`
8. `agent/kernel/session/session.go`
9. `agent/kernel/events/events.go`
10. `agent/kernel/observability/observability.go`
11. `agent/kernel/config/config.go`

## 4. 已知缺口
1. 多节点 relay 仍偏骨架，生产级故障转移与恢复细节不足。
2. 任务、事件、快照大多为内存态，重启后恢复能力有限。
3. 网关尚未提供完整认证鉴权与租户隔离。
4. 审批/审计虽可持久化，但缺少更完整治理与检索能力。
5. WebUI 控制面尚未在 `agent` 子模块内闭环交付。

## 5. 建议下一步优先级
1. 优先补齐持久化与恢复：任务终态、事件索引、relay snapshot 落盘。
2. 增强安全基线：网关鉴权、控制面写操作限流、敏感配置防泄漏。
3. 扩展接口兼容：补充更多 responses/Anthropic 兼容场景与错误映射。
4. 完成多节点实测：心跳、剔除、接力成功率、MTTR 指标化。
