# ActionAgent 全面功能测试报告（2026-03-08）

## 1. 测试目标
- 对当前阶段内核进行全链路验证：代码回归、网关功能、多 Agent 路由、真实上游 API 联通。
- 明确当前“可用能力 / 不可用能力 / 风险点”，形成阶段现状结论。

## 2. 测试范围与环境
- 项目路径：`F:\Project\My-Project\Agent\actionAgent`
- 配置文件：`agent/actionAgent.json`
- 配置关键信息（脱敏）：
  - `http_addr`: `127.0.0.1:8000`
  - `default_agent`: `default`
  - `providers[openai-main].base_url`: `http://api.yescode.cloud/v1`
  - `providers[openai-main].api_key`: `sk-****`
  - `providers[openai-main].model`: `gpt-5.3-codex`
- 测试时间：2026-03-08（UTC+8）

## 3. 测试方法
1. 代码回归测试：`go test ./...`
2. 运行态内联通测试（临时端口，避免干扰现网）：
   - `GET /healthz`
   - `POST /v1/chat/completions`
   - `POST /v1/responses`
   - `GET /metrics`
   - `agent_id` 选择优先级与非法值校验
3. 上游直连对照测试：
   - `GET {base_url}/models`
   - `POST {base_url}/chat/completions`
   - `POST {base_url}/responses`

## 4. 关键测试结果

### 4.1 代码回归（源码级）
- 命令：`go test ./...`（在可写缓存路径下执行）
- 结果：**通过**
- 结论：当前仓库源码测试集整体无回归。

### 4.2 二进制一致性检查
- 发现：仓库内已有 `agent/actionagentd.exe` 与当前源码行为不一致（旧二进制未体现最新多 Agent 校验逻辑）。
- 处理：基于当前源码重新构建 `agent/actionagentd-test.exe`，后续联通测试全部使用新构建二进制执行。
- 结论：测试结论以“源码新构建二进制”结果为准。

### 4.3 内核网关联通（新构建二进制）
- `GET /healthz`: `200`，`ready=true`。
- `POST /v1/chat/completions`（默认 agent）：HTTP `200`，业务 `state=FAILED`，`payload.error="Upstream request failed"`。
- `POST /v1/responses`（默认 agent）：HTTP `200`，业务 `state=FAILED`，`status=failed`，`payload.error="Upstream request failed"`。
- `agent_id` 行为：
  - `body.agent_id=default` 且 `X-Agent-ID=non-existent-agent`：请求可执行（命中 body 优先）。
  - 仅 `X-Agent-ID=non-existent-agent`：HTTP `400`，`validation_error`。
  - `body.agent_id=non-existent-agent`：HTTP `400`，`validation_error`。
- `GET /metrics`：出现按 agent 维度计数，示例：
  - `model_agent_default_route_fail`
  - `model_agent_default_error_format`

### 4.4 上游直连对照（同 URL / 同 API Key）
- `GET /models`: HTTP `200`，返回可用模型清单。
  - 示例模型：`gpt-5.3`, `gpt-5.3-codex`, `gpt-5.2`, `gpt-5.2-codex`, `gpt-5.1-codex` 等。
- `POST /chat/completions`: HTTP `400`，错误体：`{"error":{"message":"Upstream request failed","type":"upstream_error"}}`。
- `POST /responses`: HTTP `200`，`status=completed`，可返回正常 `response` 对象。

## 5. 结论（现阶段开发现状）

### 5.1 已完成能力
- 内核基础能力可用：配置加载、任务执行、网关暴露、指标输出。
- 多 Agent 主干能力已落地：`default_agent + agents[]`、`agent_id` 校验、按 agent 指标统计。
- 密钥治理能力已支持：`api_key` 与 `api_key_env` 两种模式。
- 默认端口统一到 `8000` 的方案已落地（配置层）。

### 5.2 当前阻塞点
- **真实模型“端到端成功应答”尚未完成**（针对当前上游 `api.yescode.cloud`）：
  - 上游 `chat/completions` 路径返回 `upstream_error`。
  - 但上游 `responses` 路径可正常完成。
- 当前内核 OpenAI 适配器统一走 `/chat/completions`，导致本地 `/v1/responses` 也被间接卡在上游 chat 路径失败。

### 5.3 现阶段判定
- 阶段状态：**核心框架完成，真实模型接入“部分完成”**。
- 具体表现：
  - “配置与路由能力”已完成。
  - “与该上游的成功推理闭环”未完成（路径协议不匹配/上游 chat 不可用）。

## 6. 风险与改进建议（按优先级）
1. 高优先级：为 OpenAI provider 增加可配置调用端点策略（`chat_completions` / `responses`），并让 `/v1/responses` 优先走上游 `/responses`。
2. 高优先级：补充上游协议适配层（字段映射、错误透传、stream 行为），避免仅靠通用消息格式硬映射。
3. 中优先级：发布流程增加“源码构建指纹/版本号”校验，避免旧 `exe` 与源码漂移。
4. 中优先级：将明文 `api_key` 从仓库配置迁移到 `api_key_env`，降低密钥泄露风险。
5. 中优先级：新增自动化 E2E 用例，分别验证 `/chat/completions` 与 `/responses` 的真实上游行为。

## 7. 附：本次执行清单（摘要）
- 代码回归：`go test ./...`（通过）
- 新构建二进制：`agent/actionagentd-test.exe`
- 内联通验证端口：`127.0.0.1:8015`
- 对照端口：上游 `http://api.yescode.cloud/v1`

## 8. 第二轮排查迭代（基于用户提供 curl）

### 8.1 差异对比（修复前）
- 用户成功请求：`POST /v1/responses`，body 为 `{"model":"gpt-5.3-codex","input":[...],"stream":true}`。
- 内核旧逻辑（OpenAI 适配器）：
  - 不区分 `/responses` 与 `/chat/completions` 语义，统一调用上游 `POST /chat/completions`。
  - 当输入为 `input:[{role,content}]` 时，被转换成单条 `messages` 文本（内容是 JSON 字符串），与用户 curl 的 `input` 结构不一致。
  - 仅设置基础请求头，未显式带 `Accept` 和固定 `User-Agent`。

### 8.2 本轮修复
- 文件：`agent/kernel/model/http_adapter.go`
- 变更：
  1. OpenAI 适配器按输入形态分流：
     - 含 `messages` -> 调上游 `/chat/completions`
     - 含 `input`（且无 `messages`）-> 调上游 `/responses`
  2. 新增 `responses` 响应解析（优先 `output_text`，兼容从 `output[]` 提取文本）。
  3. 增加默认请求头：`Accept: */*`、`User-Agent: ActionAgent/1.0`。

### 8.3 修复后验证结果（2026-03-08）
- 回归：`go test ./...` 通过。
- 复测（本地网关 -> 上游真实接口）：
  - `POST /v1/responses`（使用与用户 curl 同形态 body）：
    - HTTP `200`
    - 业务 `state=SUCCEEDED`
    - 返回 `output_text` 正常。
  - `POST /v1/chat/completions`：
    - 仍返回 `Upstream request failed`（符合上游 chat 路径当前行为）。
- 结论：
  - “你 curl 成功、内核失败”的主因已定位并修复：**旧版内核调用了错误的上游路由与请求结构**。
  - 当前 `/v1/responses` 链路已打通；`/v1/chat/completions` 的失败属于上游 chat 路径能力/兼容性问题。

## 9. 第三轮迭代（完成建议项 1 + 2）

### 9.1 建议项 1：替换实际运行二进制
- 已执行：重新编译并覆盖 `agent/actionagentd.exe`（非测试临时二进制）。
- 验证：使用新 `actionagentd.exe` 启动实例后，`/v1/responses` 已按修复逻辑生效。

### 9.2 建议项 2：`/v1/responses` SSE 透传
- 已实现能力：
  1. 网关 `POST /v1/responses` 支持 `stream=true`，进入流式分支。
  2. runtime 新增上游流式请求方法，按 agent 绑定 provider 发起 `POST {base_url}/responses` 且 `stream=true`。
  3. 网关将上游 `text/event-stream` 逐块透传给客户端（含 flush）。
- 代码位置：
  - `agent/kernel/gateway/server.go`
  - `agent/kernel/runtime.go`
  - `agent/kernel/runtime_test.go`（新增流式集成测试）
- 回归结果：`go test ./...` 通过。

### 9.3 实测结果（2026-03-08）
- 使用新 `agent/actionagentd.exe`，请求：
  - `POST /v1/responses`，body：
    - `{"model":"gpt-5.3-codex","input":[{"role":"user","content":"ping"}],"stream":true}`
- 返回：
  - HTTP `200`
  - `Content-Type: text/event-stream`
  - 可收到 `response.created` / `response.output_text.delta` / `response.completed` 事件。

---
本报告基于 2026-03-08 当日实测结果；如上游网关策略或模型权限变更，结论会随之变化。
