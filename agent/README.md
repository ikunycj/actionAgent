# Agent Module README

`agent/` 是 ActionAgent Core 的 Go 运行时模块。它负责：

- 启动 `actionagentd`
- 解析配置并监听 HTTP 端口
- 暴露 Core API、bridge、observability 接口
- 托管同包 WebUI 静态资源
- 组装 task、model、session、tools、events、storage 等内部能力

## 环境要求

- Go `1.25+`
- Node.js 和 npm
  作用：构建 `../web` 的生产静态资源

查看版本：

```powershell
go version
npm -v
```

## 目录结构

```text
agent/
  cmd/actionagentd/         进程入口
  internal/app/runtime/     启动、装配、运行时编排
  internal/app/service/     预留给更聚焦的应用服务
  internal/adapter/httpapi/ HTTP API、bridge、WebUI 静态托管
  internal/core/            task / model / dispatch / tools / session / memory
  internal/platform/        config / events / observability / storage
  actionAgent.json          本地默认配置
```

依赖方向保持为：

```text
cmd -> internal/app/runtime -> internal/adapter + internal/core + internal/platform
```

## 本地编译

只编译 Core：

```powershell
cd agent
go build -buildvcs=false -o actionagentd.exe ./cmd/actionagentd
```

只做测试：

```powershell
cd agent
go test ./...
```

## 本地运行

最常见的启动方式：

```powershell
cd agent
.\actionagentd.exe --config "$PWD\actionAgent.json"
```

默认行为：

- 用户配置的是 `port`
- Core 默认监听 `127.0.0.1:<port>`
- `--addr` 只用于高级部署覆盖

健康检查：

```powershell
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:8000/healthz"
```

## 同包 WebUI 开发与运行

开发阶段需要先构建 `web/`：

```powershell
cd ..\web
npm run build
cd ..\agent
go build -buildvcs=false -o actionagentd.exe ./cmd/actionagentd
.\actionagentd.exe --config "$PWD\actionAgent.json"
```

当 `../web/dist` 存在时，Core 会直接同源托管：

- `GET /`
- `GET /app/agents/<agent-id>/overview`

发布包场景下，Core 会优先查找与二进制同目录的 `webui/` 文件夹。

你也可以显式覆盖：

```powershell
.\actionagentd.exe --config "$PWD\actionAgent.json" --webui-dir "F:\path\to\webui"
```

## 一键生成发布包

仓库根目录执行：

```powershell
.\scripts\build-core-package.ps1
```

输出目录结构：

```text
out/core-package/
  actionagentd.exe
  actionAgent.json
  webui/
```

这样用户只需要部署一个目录，不需要再单独启动 Web 前端。

## 当前主要接口

- `GET /healthz`
- `GET /v1/runtime/agents`
- `POST /v1/run`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /ws/frame`
- `GET /events`
- `GET /metrics`
- `GET /alerts`

## 代码落点建议

新增功能时，先判断改动属于哪一层：

- 新 HTTP 接口、请求解析、响应结构
  改 `internal/adapter/httpapi/`
- 启动流程、装配关系、运行时状态
  改 `internal/app/runtime/`
- 可复用的任务、模型、会话、工具、调度逻辑
  改 `internal/core/`
- 配置、事件总线、指标、存储
  改 `internal/platform/`

推荐顺序：

1. 先把能力放进 `core` 或 `platform`
2. 再在 `app/runtime` 里完成装配
3. 最后在 `adapter/httpapi` 暴露接口或页面
4. 补测试
5. 运行 `go test ./...`

## 当前 WebUI 集成约束

- 生产发布默认走同包同源，不再把“手填 Core URL”当主路径
- WebUI 作用域以 active agent 为准
- `/v1/runtime/agents` 为 WebUI 提供默认 agent 和可用 agent 列表
- `task.list` / `task.get` 已支持 `agent_id` 过滤
- 历史、配置编辑、诊断页仍有占位内容，后续继续补齐
