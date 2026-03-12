# 06. History

## 目标
完成会话列表与 transcript 查看能力，打通首发版本的“完整历史”要求。

## 前置依赖
1. Chat 阶段完成主对话入口。
2. Core 侧历史接口可用。
接口范围：`GET /v1/sessions`、`GET /v1/sessions/{key}/transcript`
3. Core 已保证 `run`、`chat.completions`、`responses` 都能形成可读取的历史事实。

## 交付物
1. `/app/history`
2. 会话列表面板
3. transcript 查看面板
4. URL 驱动的会话选择与分页状态

## 任务清单
- [ ] 6.1 建立 `sessions` 与 `transcript` 的 RTK Query endpoint。
- [ ] 6.2 实现会话列表区域，展示会话 key 和最近活动时间。
- [ ] 6.3 实现 transcript 主视图，展示消息序列与错误结果。
- [ ] 6.4 支持按分页或游标拉取历史消息。
- [ ] 6.5 将当前选择的会话 key 同步到 URL。
- [ ] 6.6 支持从会话历史跳转回聊天页。
- [ ] 6.7 支持从历史消息或运行结果跳转到任务详情。
- [ ] 6.8 明确空状态、加载态和接口不可用时的降级提示。

## 验收标准
1. 用户可浏览最近会话列表。
2. 用户可查看指定会话的完整 transcript。
3. 页面刷新后仍能根据 URL 恢复当前会话上下文。
4. 历史页与聊天页、任务页之间的跳转关系清晰。

## 阻塞项
1. 若 `/v1/sessions*` 未就绪，本阶段只能先完成列表和 transcript 的页面壳层。
