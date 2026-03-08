# ActionAgent Core Agent Architecture

## 1. 总览
当前核心内核采用单进程单二进制架构（`actionagentd`），在一个 runtime 中统一承载：
1. HTTP/WS 网关接入
2. 任务编排与调度
3. 模型网关与工具运行时
4. 会话/记忆处理
5. 事件、指标、告警

对应主入口：
- `agent/cmd/actionagentd/main.go`
- `agent/kernel/runtime.go`

## 2. 已有模块骨架（代码目录）
1. `kernel/gateway`：HTTP 路由与 WS typed frame 入口。
2. `kernel/task`：状态机定义、lane queue、幂等去重。
3. `kernel/dispatch`：local-first 调度、heartbeat、relay snapshot。
4. `kernel/model`：Router、HTTPAdapter、错误分类、重试退避、熔断。
5. `kernel/tools`：工具注册、审批 token、审计记录与持久化。
6. `kernel/session`：会话 key 规范化、转录存储、维护策略执行。
7. `kernel/memory`：vector + FTS 检索与 FTS 降级。
8. `kernel/events`：事件总线、订阅广播、内存 sink。
9. `kernel/observability`：结构化日志、指标聚合、告警评估。
10. `kernel/config`：配置解析、校验、保存、热更新分类。
11. `kernel/storage`：KV 抽象（当前默认 in-memory）。
12. `kernel/agent_registry.go`：多 agent 注册、默认路由与解析。

## 3. 启动时序（`Runtime.Init`）
1. `config`：解析路径并加载单一配置文件。
2. `logging`：初始化结构化日志。
3. `events`：初始化事件总线。
4. `storage`：初始化 KV 存储（内存实现）。
5. `initSubsystems`：初始化 dispatcher/task/tools/session/memory。
6. `model-runtime`：根据 provider 配置构建 Router。
7. `agent-registry`：加载 enabled agents 与 default agent。
8. `gateway`：注册路由并启动 HTTP server。
9. `probe ready=true` 并发布 `startup.complete` 事件。

## 4. 请求主链路

### 4.1 HTTP 任务请求链路
1. 网关接收 `/v1/run`、`/v1/chat/completions`、`/v1/responses`。
2. 网关统一封装 `task.ExecutionEnvelope`（生成 `request_id/task_id/run_id`）。
3. Runtime 执行 `Run()`：更新指标、发布 `request.accepted` 事件。
4. Task Engine：
   - 幂等检查（`idempotency_key`）。
   - lane 并发调度（`LaneQueue.Submit`）。
   - dispatcher 选节点。
   - 调用 Runtime `Execute()` 执行模型/业务逻辑。
5. Runtime 写入成功/失败指标，并经 `TerminalAggregator` 收敛终态。
6. 网关返回标准响应。

### 4.2 执行链路（`Runtime.Execute`）
1. 解析 `agent_id`，获取对应 `AgentRuntime`。
2. 将 session key 作用域化为 `agent:<id>:...`。
3. 调用 `ModelRuntime.Route()` 执行 provider 路由与 fallback。
4. 组装 payload（provider/model/output/fallback_step/credential_id）。
5. `operation=run` 时写入会话转录。

## 5. 关键设计细节

### 5.1 任务状态机与合法迁移
状态：`CREATED`、`QUEUED`、`DISPATCHING`、`RUNNING`、`WAITING_APPROVAL`、`RETRYING`、`SUSPENDED`、`SUCCEEDED`、`FAILED`、`CANCELLED`。  
迁移规则由 `task.legalTransitions` 固定，避免跨态跳转。

### 5.2 Lane 并发策略
1. 默认 lane 并发上限来自 `queue_concurrency`。
2. `session:*` lane 强制串行（容量 1）。
3. runtime 进入 draining 后拒绝新任务入队。

### 5.3 幂等与终态聚合
1. `DedupeStore` 维护 pending/complete 两阶段状态及 TTL。
2. `idempotency_key` 命中后直接返回历史结果并标记 `replay=true`。
3. `TerminalAggregator` 保证同 `task_id` 只对外暴露一个终态。

### 5.4 模型路由与弹性
1. Router 顺序：请求显式 provider -> primary -> fallbacks 去重后遍历。
2. 凭据池支持会话粘性（sticky credential）。
3. 错误驱动冷却：`rate_limit/timeout` 短冷却，`billing` 长禁用。
4. 可重试错误执行指数退避；达到阈值触发 provider circuit open。
5. HTTPAdapter 支持 OpenAI chat/responses 与 Anthropic messages 协议映射。

### 5.5 工具审批与审计
1. L2 工具调用前必须 `ValidateAndConsume`。
2. token 绑定 `device/node/run/scope`，不匹配直接拒绝。
3. 审计记录、审批 token 可序列化落盘到 `state/tools-state.json`。

### 5.6 会话与记忆
1. 会话键通过 `session.NormalizeKey` 规范化。
2. TranscriptStore 支持 `prune_after/max_entries/max_disk` 策略。
3. Memory engine 优先 vector，失败回退 FTS，文件不存在返回空内容。

### 5.7 可观测
1. `events.Bus` 按 `run_id` 维护单调 `seq`。
2. 指标由 `observability.Metrics` 聚合并通过 `/metrics` 输出。
3. `/alerts` 根据快照阈值评估告警，不依赖外部告警系统。

## 6. 配置架构
1. 配置解析顺序：`--config` > `ACTIONAGENT_CONFIG` > `<binary-dir>/actionAgent.json` > 系统默认路径。
2. 只加载一个配置文件；支持默认配置自动落盘。
3. `UpdateConfig` 流程：
   - `Normalize + Validate`
   - 构建新 model runtime/agent registry
   - `AtomicSave`
   - `ClassifyReload` 输出 `hot|restart|noop`

## 7. 当前约束与风险
1. 任务、事件、调度快照主要为内存态，重启恢复能力有限。
2. dispatcher 的 relay 为骨架能力，尚未形成生产级多节点治理闭环。
3. 网关目前无完整认证授权层，适合内网或受控环境。
4. `/events` 仅实时订阅流，不提供分页历史查询接口。
