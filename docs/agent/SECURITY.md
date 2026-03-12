# ActionAgent 控制面安全设计

## 1. 文档目的

本文档定义 ActionAgent 控制面的安全基线。这里的“控制面”主要指：

1. 与 Core 同源交付的 WebUI 控制台
2. 用于控制、管理和调试 Agent 的 HTTP / Bridge 管理接口
3. 配置、密钥、审批、审计和高危执行相关链路

本文档参考 OpenClaw 的 `pairing`、`allowlist`、`approval`、`audit`、`secrets` 与 `sandbox policy` 思路，但收敛到 ActionAgent 当前“单 Core、单用户优先、Agent-scoped 控制台”的产品形态。

## 2. 威胁模型

### 2.1 需要保护的对象

1. Agent 配置
2. Provider 密钥与敏感参数
3. 高危工具和执行策略
4. 历史、任务和审计数据
5. Core 的网络暴露面与运行时管理能力

### 2.2 主要风险

1. 浏览器页面被当成普通聊天页，实际却拥有高权限控制能力。
2. 控制面在无认证或弱认证状态下暴露到非 loopback 地址。
3. 浏览器长期保存高权限 Token，导致会话被窃取后难以收敛风险。
4. 已登录用户对所有写操作“默认全能”，缺少高危动作二次保护。
5. 配置、日志、调试面板或审计导出中泄漏密钥原文。

## 3. 安全原则

1. 默认本地：默认只服务本机、单用户、loopback 场景。
2. 同源优先：生产环境优先使用 Core 同源托管的 WebUI。
3. 身份显式：控制台访问必须能识别 subject、device/session 和 auth mode。
4. 最小权限：权限同时受 `agent_id` 作用域和 capability scope 约束。
5. 二次保护：高危动作必须具备审批或等价的二次确认。
6. 全量审计：认证、审批、配置变更和高危执行都必须留痕。
7. 密钥不回显：密钥可写不可读回明文。

## 4. 访问模式

### 4.1 `local-paired`

这是默认模式，适用于本机控制台：

1. Core 监听 loopback 地址。
2. 首次访问时生成短时 pairing code 或 owner bootstrap secret。
3. 用户在同源 WebUI 完成配对，换取控制台会话。
4. 后续通过 Cookie 会话恢复访问。

适用场景：

1. 单机开发
2. 单用户本地部署
3. 不希望维护账号密码体系的受控环境

### 4.2 `token`

这是自动化和远程 API 的主要模式：

1. 使用 Bearer Token。
2. Token 必须携带过期时间、scope 和可撤销标识。
3. Token 主要用于 CLI、脚本、自动化或受控远程调用。

### 4.3 `trusted-proxy`

这是远程浏览器访问的推荐模式：

1. Core 不直接对公网暴露完整身份能力。
2. 由反向代理、SSO 或企业网关提供身份。
3. Core 只在显式信任代理配置下接收身份头。

### 4.4 `password`

密码模式不是默认方案。只有在明确接受本地身份存储、密码重置和会话治理复杂度时才考虑引入。

### 4.5 禁止模式

`none` 不作为正式生产控制面模式。只要监听地址不是 loopback，就不能在无认证控制面下启动。

## 5. 会话模型

### 5.1 浏览器会话

WebUI 控制台默认使用服务端会话：

1. `HttpOnly` Cookie
2. `SameSite=Lax` 或更严格
3. 有 TLS 时启用 `Secure`
4. 写操作必须附带 CSRF 令牌

原因：

1. WebUI 是高权限控制面，不应依赖前端长期保存 Access Token。
2. 同源托管可以天然降低跨域和 Token 传输复杂度。

### 5.2 API 会话

CLI 与自动化调用使用 Bearer Token。它与浏览器 Cookie 会话不是同一模型。

### 5.3 会话身份字段

每个控制面身份至少包含：

1. `subject`
2. `auth_mode`
3. `device_id` 或 `session_id`
4. `agent_scope`
5. `capability_scopes`
6. `issued_at` / `expires_at`

## 6. 授权模型

### 6.1 Agent 作用域

控制台不是全局漂浮的管理页。每个请求都必须落在明确的 Agent 上下文中：

1. 显式指定的 `agent_id`
2. 或回退到 `default_agent`

无权访问目标 Agent 的身份必须被拒绝。

### 6.2 Capability Scopes

建议至少区分以下 scope：

1. `agent:read`
2. `agent:run`
3. `agent:debug`
4. `agent:config:write`
5. `agent:secrets:write`
6. `agent:approval:resolve`
7. `runtime:admin`

不要只使用粗粒度 `admin/viewer` 来表达控制面权限。

## 7. 高危动作与审批

以下操作至少视为高危动作：

1. 配置应用
2. 密钥写入或轮换
3. 监听地址、远程暴露或代理信任配置变更
4. 打开高危工具
5. 放宽执行策略或沙箱策略

这些动作需要同时满足：

1. 已认证身份
2. 对应写权限 scope
3. 有效审批令牌或等价的后端强制二次确认

前端弹窗只能作为交互手段，不能作为唯一安全措施。最终强制必须在后端。

## 8. 审计与限流

### 8.1 审计范围

以下事件都必须进入审计：

1. 配对成功与失败
2. 登录、登出、会话恢复失败
3. scope 拒绝
4. 审批通过与拒绝
5. 配置写入与应用
6. 高危工具执行

### 8.2 审计字段

建议至少包含：

1. 时间
2. subject
3. device/session
4. `agent_id`
5. action
6. result
7. redacted payload summary

### 8.3 限流

控制面写操作按“device/session + IP”做限流，避免暴力尝试和异常批量写入。

## 9. 密钥处理

1. 密钥写入后不通过普通读接口明文返回。
2. UI 只展示“是否已设置、最后更新时间、由谁修改”等元信息。
3. 日志、审计和错误消息必须脱敏。
4. 后续优先接入系统安全存储或安全引用，而不是把密钥当普通 JSON 配置字段长期暴露。

## 10. WebUI 交互要求

1. 首次访问走配对，而不是默认账号密码。
2. 控制台顶部必须清晰显示当前 Agent、当前身份和授予的能力范围。
3. 高危动作必须明确展示“需要审批”或“将写入审计”。
4. `401`、`403`、`429` 需要可读的控制面错误提示，而不是泛化成普通网络错误。
5. 秘钥类表单提交后只能看到掩码状态，不能回填原文。

## 11. 当前差距

截至 2026-03-10，当前仓库仍处于“控制面安全模型已定义、后端 auth 仍待实现”的状态。优先缺口包括：

1. `/v1/auth/*` 真实后端接口
2. `local-paired` 首次配对流程
3. 浏览器 Cookie 会话与 CSRF
4. agent-scoped scope enforcement
5. 高危控制面动作与审批的统一约束

## 12. 后续实施顺序

1. 先冻结文档和 OpenSpec 契约。
2. 再实现 auth mode 与启动校验。
3. 再实现 pairing、session、scope、approval、audit。
4. 最后清理浏览器直存 Token 的旧假设。
