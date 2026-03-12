# 02. Connection And Auth

## 目标
完成 Core 连接、登录、认证态维持和路由守卫，为后续配置和对话能力建立访问前提。

## 前置依赖
1. Foundation 阶段完成。
2. Core 侧认证接口可用。
接口范围：`POST /v1/auth/login`、`POST /v1/auth/refresh`、`POST /v1/auth/logout`、`GET /v1/auth/me`

## 交付物
1. `/connect` 页面
2. `/login` 页面
3. `RequireConnection`、`RequireAuth`、`RequireAdmin`
4. 认证状态管理与自动刷新机制

## 任务清单
- [x] 2.1 实现 Core Base URL 输入、保存和切换逻辑。
- [x] 2.2 接入 `GET /healthz`，完成连接检测和错误提示。
- [x] 2.3 实现登录表单，支持 token/password 提交。
- [x] 2.4 接入 `me`、`refresh`、`logout`，完成会话维持。
- [x] 2.5 设计并实现 `401 -> refresh -> retry -> fail back to login` 的全局流程。
- [x] 2.6 在 Store 中维护 `accessToken`、`refreshToken`、`role`、`actor`。
- [x] 2.7 实现连接守卫、认证守卫和管理员守卫。
- [x] 2.8 完成顶栏身份态、登录态、登出入口。
- [x] 2.9 明确 `viewer` 与 `admin` 的前端可见边界。

## 验收标准
1. 用户可从 `/connect` 完成 Core 地址配置并通过健康检查。
2. 用户可完成登录，并在刷新页面后保持认证态。
3. 认证失效时，前端能自动刷新或回退到登录页。
4. 非管理员无法进入配置写页面。

## 阻塞项
1. 若 `/v1/auth/*` 未就绪，本阶段只能先完成页面和状态机壳层，不能闭环联调。

## 联调说明
1. 本阶段前端壳层、状态机、自动刷新和权限守卫已按约定接口落地，并通过本地测试验证。
2. 真实登录、刷新和 `me/logout` 闭环仍依赖 Core 补齐 `/v1/auth/*` 后再做联调确认。
