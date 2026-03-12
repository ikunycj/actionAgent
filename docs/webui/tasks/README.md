# ActionAgent WebUI Tasks

## 1. 目录说明

本目录用于承载 WebUI 的执行任务拆分，按阶段组织，而不是把所有任务堆叠在一张长文档里。

目录约定：

1. `todo/`：未开始或进行中的任务文档
2. `archive/`：已完成并归档的任务文档

## 2. 执行顺序

已完成并归档：

1. `archive/01-foundation.md`
2. `archive/02-connection-auth.md`
3. `archive/07-task-center.md`

建议按以下顺序继续推进：

1. `todo/03-shell-overview.md`
2. `todo/04-config-editor.md`
3. `todo/05-chat.md`
4. `todo/06-history.md`
5. `todo/08-quality-release.md`

## 3. 后端依赖

以下任务依赖 Core 补齐接口后才能联调闭环：

1. 认证：`/v1/auth/*`
2. 配置：`/v1/config/*`
3. 历史：`/v1/sessions`、`/v1/sessions/{key}/transcript`

以下任务可基于现有后端能力先行联调：

1. 连接检查：`GET /healthz`
2. 对话主链路：`POST /v1/responses`
3. 任务中心：`POST /ws/frame` + `task.list` / `task.get`

## 4. 维护规则

1. 每个任务文档只描述一个阶段的交付目标、依赖、清单和验收标准。
2. 阶段完成后，将对应文件从 `todo/` 移动到 `archive/`。
3. 若某个阶段因后端接口未就绪而阻塞，直接在对应任务文档中补充阻塞项，不改动既定顺序。
4. 若范围变更，应先更新 `docs/webui/PRD.md` 或 `docs/webui/ARCHITECTURE.md`，再改动任务文档。

## 5. 关联文档

1. `docs/webui/PRD.md`
2. `docs/webui/ARCHITECTURE.md`
3. `docs/agent/PRD.md`
4. `docs/agent/API.md`
5. `docs/agent/CURRENT.md`
