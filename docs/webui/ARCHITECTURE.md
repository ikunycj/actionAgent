# ActionAgent WebUI Architecture

## 1. 文档信息

- 模块：`docs/webui`
- 文档类型：架构设计
- 版本：v0.3
- 日期：2026-03-11
- 上位文档：
  - `docs/webui/PRD.md`
  - `docs/PRD.md`

## 2. 总览

WebUI 使用 React + TypeScript + Vite + Tailwind + Redux。当前交付方式如下：

1. 开发态：`web/` 仍可独立运行和构建
2. 生产态：构建产物进入 Core 发布包，由 Core 同源提供，根路径 `/` 即为 WebUI 入口

因此，WebUI 的架构目标不是“长期独立部署的前端产品”，而是“随 Core 交付的控制台”。

## 3. 运行拓扑

生产态拓扑：

```text
Browser
   |
   v
Core same-origin host
   |- WebUI static assets
   |- /healthz
   |- /v1/*
   |- /ws/frame
   |- /events /metrics /alerts
```

开发态拓扑：

```text
Vite dev server
   |
   +-- same-origin mock or proxy
   |
   +-- remote Core for debugging
```

## 4. 关键架构决策

### 4.1 生产同源优先

1. 生产环境默认同源访问 Core。
2. 浏览器不再把“手工输入 Core Base URL”作为主路径。
3. CORS 只作为开发或特殊部署兼容能力。

### 4.2 Agent-scoped 控制台

1. WebUI 首先进入某个 Agent 的控制台，而不是漂浮的全局前端壳。
2. 配置、状态、历史和任务视图默认围绕当前 Agent 组织。
3. 如果 Core 暴露多个 Agent，前端应把 agent 选择和切换视为控制台能力，而不是顶层编排平台。

### 4.3 控制面而非主入口

1. WebUI 优先承载配置和诊断。
2. 客户端优先承载高频对话和日常操作。
3. 即使保留对话页，也应视为调试工具，而不是主产品入口。

## 5. 前端结构

代码组织继续保持：

```text
web/
  src/
    app/
    shared/
    features/
    widgets/
    pages/
```

边界约束：

1. `app` 负责启动、路由和全局 Provider。
2. `shared` 负责 API、类型、基础组件和工具。
3. `features` 负责业务能力。
4. `widgets` 负责页面级装配。
5. `pages` 负责路由入口。

## 6. 运行时要求

1. 生产态必须支持 same-origin API 访问。
2. 开发态必须支持独立调试。
3. WebUI 必须能从运行时解析 Agent 资源清单，并在需要时切换当前 Agent。
4. WebUI 必须能区分管理操作与普通查看。

## 7. 发布路径

1. `web/` 先构建出静态产物。
2. 产物进入 `agent` 发布流程。
3. Core 在生产态提供静态资源和 SPA fallback。
4. API 路由优先级必须高于前端 fallback。
