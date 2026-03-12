# ActionAgent Core API

## 1. 范围

本文档描述当前已经落地、并且被 Bundled WebUI 或客户端直接依赖的 Core API。

## 2. HTTP 接口

### 2.1 Runtime And Delivery

- `GET /`
  用途：返回同包 WebUI 首页
- `GET /healthz`
  用途：健康检查
- `GET /v1/runtime/agents`
  用途：返回 default agent 和可用 agent 列表，供 WebUI 展示 Core 运行范围与 agent 资源清单

### 2.2 Task And Model

- `POST /v1/run`
  用途：通用任务执行
- `POST /v1/chat/completions`
  用途：OpenAI Chat Completions 风格接口
- `POST /v1/responses`
  用途：OpenAI Responses 风格接口

### 2.3 Observability

- `GET /events`
- `GET /metrics`
- `GET /alerts`

## 3. Bridge 接口

- `POST /ws/frame`

当前已实现方法：

- `agent.run`
- `agent.wait`
- `task.get`
- `task.list`
- `audit.query`
- `approval.list`
- `session.stats`
- `session.maintain`
- `observability.alerts`

## 4. Agent 目标选择约定

### 4.1 选择优先级

- `body.agent_id`
- `X-Agent-ID`
- `default_agent`

### 4.2 WebUI 依赖方式

- `/v1/runtime/agents` 用于读取 Core 运行范围与 agent 资源清单
- 任务、历史、配置等接口在支持时可附带 `agent_id` 作为筛选或下钻参数
- Core 级总览页不应依赖固定 active-agent 路由才能工作

### 4.3 当前结果

- task 查询已经具备向 agent 维度收口的能力
- WebUI 当前页面不应被限定为必须先进入单一 agent 路由

## 5. 当前缺口

以下能力仍未完整交付：

- `/v1/auth/*`
- `/v1/config/*`
- session 列表接口
- transcript 读取接口
- 更完整的支持 Core 级与 agent-scoped 双层语义的 history/config API
