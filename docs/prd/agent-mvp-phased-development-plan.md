# ActionAgent MVP 分阶段开发任务与完整功能说明

## 0. 文档信息
- 日期：2026-03-08
- 适用范围：`actionAgent/agent`
- 目标：统一描述分阶段任务进度，并给出 MVP 最终功能口径

## 1. 路线图总览

| 阶段 | 名称 | 当前状态 | 目标结果 |
|---|---|---|---|
| 阶段 0 | 内核基线落地 | 已完成 | 单机内核可运行，API 主链路闭环，基础测试覆盖 |
| 阶段 1 | 真模型接入与密钥治理 | 已完成（迭代5） | 支持 URL + APIKey + API 规范（OpenAI/Anthropic） |
| 阶段 2 | 协议增强与生产稳态 | 已完成（迭代4） | 完成协议扩展、稳态能力、观测告警与回归矩阵 |
| 阶段 3 | MVP 收敛与发布准备 | 待开始 | 交付发布清单、运维手册、安全基线与验收报告 |

## 2. 阶段 0（已完成）：内核基线落地

来源：`openspec/changes/archive/2026-03-08-implement-agent-kernel-core/tasks.md`（全量已勾选）

1. `[x]` 启动链路：`config -> logging -> events -> storage -> gateway`，并支持 fail-fast
2. `[x]` API 入口：`/v1/chat/completions`、`/v1/run`、`/ws/frame`、`/events`、`/metrics`
3. `[x]` 任务引擎：状态迁移、lane 并发、幂等去重、draining
4. `[x]` 调度与接力：local-first、relay 候选、终态聚合
5. `[x]` 模型网关骨架：adapter 接口、primary/fallback、错误分类与冷却
6. `[x]` 工具运行时：L0/L1/L2 分级、审批令牌校验、审计记录
7. `[x]` 会话与记忆：session key 规范、转录存储、FTS-only 降级
8. `[x]` 配置系统：单源加载、原子写、重载计划分类
9. `[x]` 观测体系：事件 schema、结构化日志、基础指标
10. `[x]` 测试与文档：单测/组件/集成/失败路径测试、README 更新

## 3. 阶段 1（已完成）：真模型接入与密钥治理

1. `[x]` 扩展配置结构：provider、api_style、base_url、model、timeout、max_attempts、api_key_env/api_key
2. `[x]` 密钥治理：支持环境变量读取密钥，不在日志中输出密钥
3. `[x]` OpenAI-compatible HTTP Adapter：真实请求、鉴权头、响应解析
4. `[x]` Anthropic-compatible HTTP Adapter：真实请求、字段映射、响应解析
5. `[x]` 路由落地：primary/fallback 绑定真实 provider 调用结果
6. `[x]` 错误分类联动：429/超时/401/格式/模型不存在 等映射
7. `[x]` API 最小兼容：`/v1/chat/completions` 返回最小兼容结构
8. `[x]` 可观测增强：provider/fallback/error_class 路由指标
9. `[x]` 集成测试：mock provider 成功/失败分类与降级路径
10. `[x]` 验证脚本：提供模型链路验证脚本（PowerShell/Bash）

## 4. 阶段 2（已完成）：协议增强与生产稳态

1. `[x]` 增加 `/v1/responses` 入口并统一执行信封
2. `[x]` WS 控制面方法增强：`agent.run/agent.wait/task.get/task.list`
3. `[x]` 出站调用稳态：重试退避、熔断、超时预算
4. `[x]` relay 稳定性：心跳阈值、快照恢复、重入防抖
5. `[x]` 审批/审计持久化：从内存升级到持久化与检索
6. `[x]` 会话治理：清理策略、配额限制、磁盘预算控制
7. `[x]` 观测告警：阈值告警评估与查询输出
8. `[x]` 回归矩阵：并发、长任务、节点波动、配置热更新场景

## 5. 阶段 3（待开始）：MVP 收敛与发布准备

1. `[ ]` 发布清单：二进制、默认配置模板、启动脚本、版本说明
2. `[ ]` 运维手册：部署、升级、回滚、故障排查、容量建议
3. `[ ]` 安全基线：默认安全配置、敏感字段脱敏、最小权限策略
4. `[ ]` 验收报告：功能/性能/稳定性验收与已知限制
5. `[ ]` MVP 关口评审：确认发布范围与延期项

## 6. MVP 最终功能口径

1. 启动与配置：
   - 固定优先级解析配置路径，支持自动初始化
   - 配置变更原子落盘与重载计划（hot/restart/noop）

2. API 与协议：
   - HTTP：`/healthz`、`/v1/chat/completions`、`/v1/run`、`/v1/responses`、`/alerts`
   - WS：req/res/event，含任务、审计、会话、观测查询方法

3. 执行引擎：
   - 状态机、lane 并发、幂等去重、draining
   - 调度与接力的终态收敛

4. 模型网关：
   - OpenAI/Anthropic 兼容适配器
   - `base_url + api_key_env/api_key` 配置
   - primary/fallback、错误分类、重试退避、熔断、凭据冷却

5. 工具与审批：
   - L0/L1/L2 分级执行
   - 审批令牌（once/always）与审计持久化检索

6. 会话与记忆：
   - 会话隔离策略与转录维护
   - 检索降级与预算治理

7. 可观测与运维：
   - 生命周期事件、结构化日志、核心指标
   - 告警阈值评估与回归验证脚本

## 7. 下一步建议

建议切入阶段 3：
1. 先做发布清单与运维手册
2. 再做安全基线与验收报告
3. 最后做 MVP 关口评审与发布决策
