# ActionAgent Agent 内核现状与目标（更新版）

## 0. 文档信息
- 日期：2026-03-08
- 范围：`actionAgent/agent`
- 说明：本文件替代早期“阶段 1 未完成”版本，反映当前最新实现状态

## 1. 当前结论

当前内核**已经支持**通过 `URL + API Key + API 规范` 与真实大模型通信。

已支持：
1. `api_style=openai`（OpenAI-compatible）
2. `api_style=anthropic`（Anthropic-compatible）
3. `base_url + api_key_env/api_key + model + timeout + retry` 配置驱动
4. primary/fallback 路由、错误分类、重试退避、熔断与凭据冷却

## 2. 当前模块现状（摘要）

1. 网关：
   - `GET /healthz`
   - `POST /v1/chat/completions`
   - `POST /v1/run`
   - `POST /v1/responses`
   - `GET /metrics`
   - `GET /alerts`
   - `POST /ws/frame`

2. 执行与调度：
   - 任务状态机、lane 并发、幂等去重、draining
   - local-first 调度与 relay 快照恢复

3. 模型网关：
   - 真实 HTTP provider adapter（OpenAI/Anthropic）
   - 错误分类（rate_limit/timeout/auth/billing/model_not_found/format）
   - primary/fallback、重试退避、熔断

4. 审批与会话治理：
   - 审批令牌与工具审计持久化
   - 会话策略（prune/max entries/max disk）与统计/维护能力

5. 可观测：
   - 结构化日志、事件流、指标
   - 阈值告警评估与查询输出

## 3. 目标功能（MVP 口径）

1. 可配置、可运行、可观测、可审计的单机 Agent 内核
2. 对外兼容主流模型 API 规范（OpenAI/Anthropic）
3. 对内稳定执行与可恢复（任务 + 调度 + 治理）
4. 为阶段 3 发布收敛提供完整工程资产（脚本、文档、验收）

## 4. 后续重点（阶段 3）

1. 发布清单与运维手册
2. 安全基线与验收报告
3. MVP 关口评审与发布决策
