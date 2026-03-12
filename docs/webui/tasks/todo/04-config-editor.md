# 04. Config Editor

## 目标
完成 Core 级模型网关与关键运行参数配置闭环，包括读取、编辑、校验、应用和回滚，并明确区分全局字段与 agent-scoped 字段。

## 前置依赖
1. Connection And Auth 阶段完成管理员守卫。
2. Core 侧配置接口可用。
接口范围：`GET /v1/config`、`PATCH /v1/config`、`POST /v1/config/apply`、`POST /v1/config/rollback`

## 交付物
1. `/app/settings/model` 页面
2. 配置草稿状态管理
3. 配置应用与回滚交互

## 任务清单
- [ ] 4.1 读取脱敏配置，并映射为前端可编辑的 Core 配置视图模型。
- [ ] 4.2 为共享模型运行时和 Core 级参数建立配置表单。
- [ ] 4.3 覆盖 P0 字段：`api_style`、`base_url`、`model`、`api_key`、`timeout`、`retry`、`stream`、`thinking_level`。
- [ ] 4.4 维护 `baseVersion`、dirty 状态、冲突状态和 applying 状态。
- [ ] 4.5 实现 `PATCH -> apply` 提交流程。
- [ ] 4.6 映射并展示后端结构化校验错误。
- [ ] 4.7 处理脱敏字段交互，避免未修改的 `api_key` 被空值覆盖。
- [ ] 4.8 对 `reload_plan=restart` 给出显式提示。
- [ ] 4.9 提供配置回滚入口。
- [ ] 4.10 为 `viewer` 提供只读展示或禁止进入。

## 验收标准
1. 管理员可查看并编辑 Core 级模型与关键运行参数配置。
2. 应用配置时能正确处理版本冲突、校验失败和重启提示。
3. API Key 等敏感字段不会被前端误清空或误持久化。
4. 回滚操作完成后，界面状态与后端最新配置重新对齐。

## 阻塞项
1. 若 `/v1/config/*` 未就绪，本阶段只能先完成 UI 草稿层和本地校验。
