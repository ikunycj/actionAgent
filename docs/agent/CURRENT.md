# Agent Core Current Status

## 1. 当前能力

Agent Core 已经具备以下可运行能力：

- 单进程启动
- 配置加载
- model routing
- task execution
- session maintenance
- events / metrics / alerts
- OpenAI-compatible HTTP API
- bridge API
- 同源托管 Bundled WebUI
- runtime agent catalog

## 2. 当前目录基线

```text
agent/
  cmd/actionagentd/
  internal/app/runtime/
  internal/adapter/httpapi/
  internal/core/
  internal/platform/
```

旧的 `kernel` 兼容层已不再作为运行时主路径。

## 3. 当前 WebUI 集成状态

- Core 会优先托管与二进制同目录的 `webui/`
- 开发阶段会回落到仓库里的 `web/dist`
- 非 API 路由走 SPA fallback
- `/healthz`、`/v1/*`、`/ws/*`、`/events`、`/metrics`、`/alerts` 不会被前端路由吞掉

## 4. 当前缺口

- auth 仍未完成
- config control plane 仍未完成
- transcript/history API 仍未完成
- 更多真实 agent 管理页仍待补齐
