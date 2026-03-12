# ActionAgent Core Agent Architecture

## 1. 总览

当前 Core 采用单进程单二进制架构，由 `actionagentd` 统一承载：

1. HTTP / Bridge / Observability API
2. 任务执行与调度
3. 模型网关与工具运行时
4. 会话、记忆、审计和配置
5. 生产态 WebUI 静态资源提供

新的目标边界是：

1. Core 既是 API 宿主，也是 WebUI 交付宿主
2. Bundled WebUI、Native Client、CLI/脚本共享同一 Core
3. Native Client 是主入口，WebUI 是管理台

## 2. 外部交互关系

```text
Native Client / Bundled WebUI / CLI
                |
                v
     HTTP / WS / Events / Metrics
                |
                v
             Agent Core
                |
      +---------+---------+
      |         |         |
      v         v         v
    Task      Model      Tools
   Engine    Gateway    Runtime
      |         |         |
      +----+----+----+----+
           v         v
        Session   Observability
        /Memory
```

## 3. 当前目录映射

1. `agent/cmd/actionagentd/main.go`
2. `agent/internal/app/runtime/*`
3. `agent/internal/adapter/httpapi/server.go`
4. `agent/internal/core/task/*`
5. `agent/internal/core/dispatch/*`
6. `agent/internal/core/model/*`
7. `agent/internal/core/tools/*`
8. `agent/internal/core/session/*`
9. `agent/internal/core/memory/*`
10. `agent/internal/platform/config/*`
11. `agent/internal/platform/events/*`
12. `agent/internal/platform/observability/*`
13. `agent/internal/platform/storage/*`

## 4. 启动时序

`Runtime.Init` 的主要顺序仍然是：

1. `config`
2. `logging`
3. `events`
4. `storage`
5. `dispatch / task / tools / session / memory`
6. `model runtime`
7. `agent registry`
8. `gateway`

后续为了支持同包 WebUI，需要在 gateway 侧补入：

1. WebUI 静态资源路由
2. SPA fallback
3. API 路由优先级保护

## 5. 请求主链路

### 5.1 API 链路

1. 请求进入 `internal/adapter/httpapi`
2. 适配层解析协议并归一化为内部执行信封
3. 运行时解析目标 Agent
4. task / model / tools / session 等能力协作完成执行
5. 结果返回给客户端、WebUI 或自动化调用方

### 5.2 Bundled WebUI 链路

1. 浏览器访问 Core 同源地址
2. Core 返回 WebUI 静态资源
3. WebUI 通过同源 API 调用 Core
4. WebUI 在运行时先解析 Core 控制面上下文与资源清单
5. 配置、状态、历史和任务视图默认围绕 Core 展开，并在需要时进入 agent 级下钻

## 6. 关键架构原则

1. API 与静态资源共存，但 API 路由优先
2. 配置事实源始终在 Core
3. WebUI 使用同源访问作为生产默认
4. 客户端与 WebUI 共享 Core 能力，但承担不同产品职责
5. WebUI 管理台首先管理整个 Core，并在需要时进入 agent 作用域，而不是预设成单 agent 控制台

## 7. 当前架构缺口

1. 还没有 WebUI 静态资源托管实现
2. 还没有生产态 SPA fallback 设计
3. 还没有完整的 Core 资源清单与 agent 级下钻管理模型
4. 认证、配置、历史和任务闭环还未完整打通
