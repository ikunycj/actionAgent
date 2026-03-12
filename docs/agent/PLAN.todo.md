# 状态说明

本文档中的任务拆分基于旧的“独立 WebUI”产品假设，现已不再作为当前主计划使用。

当前请优先参考：

- `docs/PRD.md`
- `docs/agent/PRD.md`
- `docs/webui/PRD.md`
- `openspec/changes/bundle-webui-with-core-release/tasks.md`

以下内容保留为历史参考，不应作为新的实施基线。

# ActionAgent Core Agent Plan TODO

## 1. 文档信息
- 模块：`docs/agent`
- 类型：阶段任务规划
- 日期：2026-03-10
- 依据：
  - `docs/PRD.md`
  - `docs/agent/PRD.md`
  - `docs/agent/CURRENT.md`
  - `docs/CURRENT.md`

## 2. 规划原则
1. 优先解除独立 WebUI 的首发阻塞。
2. 保持 Core 的 API-only 边界，不把前端职责拉回 Core。
3. 先做可联调、可验收、可发布的能力，再做更深的持久化与多节点增强。
4. 每一阶段都要求同时补测试和文档，而不是后补。

## 3. 当前已具备基础
- [x] Core 单二进制启动
- [x] `/healthz`
- [x] `/v1/run`
- [x] `/v1/chat/completions`
- [x] `/v1/responses`
- [x] `/ws/frame`
- [x] `task.get`
- [x] `task.list`
- [x] `agent.wait`
- [x] `/events`、`/metrics`、`/alerts`
- [x] OpenAI / Anthropic provider 路由
- [x] 基础 transcript store、memory、audit、approval 能力

## 4. 当前核心缺口
- [ ] 认证接口未实现
- [ ] 配置控制接口未实现
- [ ] 会话列表接口未实现
- [ ] transcript 历史读取接口未实现
- [ ] `chat.completions` / `responses` 未完整写入可读历史
- [ ] 独立 WebUI 联调所需的稳定 API 契约未冻结
- [ ] 二进制发布、签名、发布说明未闭环

## 5. 阶段任务规划

### Phase 1: 认证与访问控制
目标：让 Core 成为可安全暴露给独立 WebUI 的 API 服务。

- [ ] 1.1 设计 `auth.mode=token|password|trusted-proxy|none` 的配置结构与默认行为。
- [ ] 1.2 实现认证中间层，覆盖 HTTP API 与 `/ws/frame`。
- [ ] 1.3 实现 `POST /v1/auth/login`。
- [ ] 1.4 实现 `POST /v1/auth/refresh`。
- [ ] 1.5 实现 `POST /v1/auth/logout`。
- [ ] 1.6 实现 `GET /v1/auth/me`。
- [ ] 1.7 落地 `admin` / `viewer` 两级权限模型。
- [ ] 1.8 增加鉴权失败限流、锁定窗口、失败审计。
- [ ] 1.9 为 WebUI 独立部署场景明确 token 传输策略。
说明：
推荐优先做 Bearer Token 模式，避免一开始就把跨域 Cookie 复杂度引进来。

### Phase 2: 配置控制面
目标：让 WebUI 和客户端不需要登录服务器改配置文件。

- [ ] 2.1 为配置增加 `config_version` / `etag` 版本标识。
- [ ] 2.2 实现 `GET /v1/config`，默认脱敏敏感字段。
- [ ] 2.3 实现 `PATCH /v1/config`，支持 merge patch。
- [ ] 2.4 实现 `POST /v1/config/apply`。
- [ ] 2.5 实现 `POST /v1/config/rollback`。
- [ ] 2.6 实现配置 schema 校验错误的结构化返回。
- [ ] 2.7 实现配置变更审计：`actor`、`before_hash`、`after_hash`、`diff`、`ts`。
- [ ] 2.8 打通 `hot|restart|noop` reload plan 返回。
- [ ] 2.9 为配置接口补单测、集成测试与错误路径测试。

### Phase 3: 会话与历史 API
目标：支撑独立 WebUI 的“完整对话历史”能力。

- [ ] 3.1 统一 transcript 事实模型，明确消息、事件、任务结果的落盘结构。
- [ ] 3.2 让 `chat.completions` 写入 transcript。
- [ ] 3.3 让 `responses` 写入 transcript。
- [ ] 3.4 复核 `run` 路径 transcript 内容，避免与对话事实模型割裂。
- [ ] 3.5 实现会话列表接口。
- [ ] 3.6 实现指定会话 transcript 读取接口。
- [ ] 3.7 为 transcript 读取增加分页 / 游标能力。
- [ ] 3.8 为会话列表提供最近活跃时间、entry 数、最近任务等摘要字段。
- [ ] 3.9 明确 HTTP 风格与 WS bridge 风格的最终接口方案，并冻结契约。
- [ ] 3.10 补历史接口的单测和回归测试。

### Phase 4: 任务与状态接口增强
目标：让独立 WebUI 可以稳定显示任务列表、详情、回执状态。

- [ ] 4.1 审查 `task.list` 输出字段是否足够支撑 WebUI 列表。
- [ ] 4.2 为任务详情补充稳定字段约定，例如错误原因、时间戳、输出摘要。
- [ ] 4.3 明确任务列表过滤参数，例如 `limit`、`state`、`agent_id`、`session_key`。
- [ ] 4.4 明确事件流与任务状态之间的映射规则，供 WebUI 做实时刷新。
- [ ] 4.5 补统一错误码和前端可消费的错误结构。
- [ ] 4.6 补任务接口的契约测试文档。

### Phase 5: 独立 WebUI 联调准备
目标：让 `web/` 目录下的独立 WebUI 可以顺利联调和部署。

- [ ] 5.1 明确 Core 对跨域部署的 CORS 策略。
- [ ] 5.2 明确 WebUI 连接 Core 的环境变量 / 配置项约定。
- [ ] 5.3 输出本地联调方式：`web dev server -> core api`。
- [ ] 5.4 输出生产部署方式：同域反代、跨域直连、反向代理推荐方案。
- [ ] 5.5 提供独立 WebUI 的最小联调验收清单。

### Phase 6: 二进制发布闭环
目标：让用户真正走“下载即用”的路径，而不是仓库开发路径。

- [ ] 6.1 完成 Windows / Linux / macOS 预编译发布。
- [ ] 6.2 生成 `checksums`。
- [ ] 6.3 增加签名文件或签名流程。
- [ ] 6.4 完善版本命令与版本元数据注入。
- [ ] 6.5 补充发布说明、升级说明、回滚说明。
- [ ] 6.6 用发布产物而不是 `go build` 路径重新验证 README。

### Phase 7: 持久化与恢复增强
目标：让 Core 从“内存态 MVP”走向更可恢复的运行时。

- [ ] 7.1 为任务终态增加持久化存储。
- [ ] 7.2 为 transcript 增加稳定的落盘与恢复策略。
- [ ] 7.3 为事件提供索引或快照能力，而不只是实时流。
- [ ] 7.4 为 relay snapshot 增加持久化与恢复能力。
- [ ] 7.5 为重启恢复补集成测试。

### Phase 8: 文档与验证收口
目标：确保模块文档、接口文档、测试验证三者一致。

- [ ] 8.1 保持 `docs/agent/API.md` 与真实接口实现同步。
- [ ] 8.2 保持 `docs/agent/CURRENT.md` 与当前状态同步。
- [ ] 8.3 在独立 WebUI 首发前输出一份 Core 验收清单。
- [ ] 8.4 为每个 P0 接口准备 curl / PowerShell 验证示例。
- [ ] 8.5 在 README 中同步发布路径、部署方式和限制说明。

## 6. 推荐执行顺序
1. Phase 1：认证与访问控制
2. Phase 2：配置控制面
3. Phase 3：会话与历史 API
4. Phase 4：任务与状态接口增强
5. Phase 5：独立 WebUI 联调准备
6. Phase 6：二进制发布闭环
7. Phase 7：持久化与恢复增强
8. Phase 8：文档与验证收口

## 7. 当前最值得先做的 5 项
- [ ] P0-1 实现 `POST /v1/auth/login`、`GET /v1/auth/me`
- [ ] P0-2 实现 `GET /v1/config`、`PATCH /v1/config`、`POST /v1/config/apply`
- [ ] P0-3 让 `chat.completions` / `responses` 落完整 transcript
- [ ] P0-4 实现会话列表与 transcript 读取接口
- [ ] P0-5 定义独立 WebUI 的 CORS / 鉴权 / Base URL 联调方案

## 8. 暂不纳入本轮的事项
- [ ] Core 内嵌 WebUI
- [ ] 多租户 / 团队协作
- [ ] iOS 客户端
- [ ] 云端 Core 直接调用客户端本地设备能力
