# ActionAgent Current Status

## 1. 当前结论

截至 2026-03-10，ActionAgent 已经从“后端与 WebUI 分离”的方向切到“Core 同包交付 WebUI、客户端作为主入口”的方向，并且这一方向已经有实际代码落地。

当前代码状态：

- Core 可以同源托管已构建的 WebUI 静态资源
- WebUI 主路径不再要求用户手工输入 Core Base URL
- WebUI 已进入按 agent 作用域管理的路由结构
- 发布包已经可以产出 `actionagentd + webui/` 的单目录交付形态

## 2. 已落地能力

### 2.1 Core

- `actionagentd` 可正常启动
- `/healthz`
- `/v1/run`
- `/v1/chat/completions`
- `/v1/responses`
- `/ws/frame`
- `/events`
- `/metrics`
- `/alerts`
- `/v1/runtime/agents`

### 2.2 Bundled WebUI

- Core 在同源下提供 `/`
- SPA 路由支持 `/app/agents/<agent-id>/...`
- WebUI 会从 `/v1/runtime/agents` 解析 default agent 和可用 agent 列表
- task 视图已按 `agent_id` 过滤
- chat 入口已降级为 diagnostic surface

### 2.3 发布链路

- 已新增 `scripts/build-core-package.ps1`
- 已新增 `scripts/build-core-package.sh`
- 发布目录包含：
  - `actionagentd`
  - `actionAgent.json`
  - `webui/`

## 3. 仍未完成的部分

- 认证接口仍未完整交付
- 配置读写控制面仍未交付
- 历史会话列表与 transcript 读取仍未交付
- Native Client 仍以产品定义为主，尚未进入完整实现阶段

## 4. 当前推荐工作顺序

1. 补齐 auth / config / history API
2. 继续把 WebUI 占位页替换成真实管理能力
3. 推进 Native Client 成为主聊天与任务入口
4. 在此基础上再做更完整的发布和升级体验
