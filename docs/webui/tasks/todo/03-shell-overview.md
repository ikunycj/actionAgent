# 03. Shell And Overview

## 目标
完成 WebUI 的全局壳层、主导航和 Overview 页面，建立面向整个 Core 控制面的统一交互框架。

## 前置依赖
1. Foundation 阶段完成。
2. Connection And Auth 阶段至少完成连接态与登录态骨架。

## 交付物
1. `AppShell`
2. 左侧主导航
3. 顶栏状态区
4. `/app/overview` 页面

## 任务清单
- [ ] 3.1 实现统一布局：Sidebar + Topbar + Content Outlet。
- [ ] 3.2 实现主导航入口：Overview、Chat、History、Tasks、Settings。
- [ ] 3.3 实现全局反馈层：Toast、Confirm Dialog、Error Boundary。
- [ ] 3.4 在顶栏展示当前 Core 地址、认证身份、健康状态。
- [ ] 3.5 实现 Overview 页卡片布局。
- [ ] 3.6 汇总展示健康状态、认证状态、配置就绪度和运行范围摘要。
- [ ] 3.7 提供通往 Chat、History、Tasks、Settings 的快捷入口。
- [ ] 3.8 完成窄屏下的导航折叠策略。

## 验收标准
1. 所有受保护页面都运行在统一 App Shell 中。
2. Overview 可作为登录后的默认 Core 控制面落点页。
3. 全局状态信息可在顶栏稳定展示。
4. 窄屏下导航仍可访问，不出现主交互丢失。
