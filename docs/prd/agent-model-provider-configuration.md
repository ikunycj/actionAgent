# Agent 模型网关配置规范（URL + API Key + API 规范）

## 1. 目标
本文档说明如何在 `actionAgent.json` 中配置：
1. Provider 的 `base_url`
2. API 密钥来源（`api_key_env` 或 `api_key`）
3. API 规范（`api_style=openai|anthropic`）

## 2. 配置字段说明

`model_gateway.providers[]` 字段：
1. `name`：Provider 名称（内部路由标识）。
2. `api_style`：协议规范，当前支持：
   - `openai`
   - `anthropic`
3. `base_url`：Provider 基础 URL。
4. `api_key_env`：环境变量名（推荐）。
5. `api_key`：明文密钥（不推荐，仅测试场景）。
6. `model`：默认模型名称。
7. `timeout_ms`：单次 Provider 调用超时（毫秒）。
8. `max_attempts`：单 Provider 内部重试次数。
9. `max_tokens`：Anthropic 场景可选。
10. `enabled`：是否启用该 Provider。

`model_gateway` 顶层字段：
1. `primary`：首选 Provider 名称。
2. `fallbacks`：备选 Provider 顺序列表。
3. `providers`：Provider 详细配置列表。

## 3. 推荐配置示例

```json
{
  "model_gateway": {
    "primary": "openai-main",
    "fallbacks": ["anthropic-backup"],
    "providers": [
      {
        "name": "openai-main",
        "api_style": "openai",
        "base_url": "https://api.openai.com/v1",
        "api_key_env": "ACTIONAGENT_OPENAI_API_KEY",
        "model": "gpt-4o-mini",
        "timeout_ms": 20000,
        "max_attempts": 2,
        "enabled": true
      },
      {
        "name": "anthropic-backup",
        "api_style": "anthropic",
        "base_url": "https://api.anthropic.com/v1",
        "api_key_env": "ACTIONAGENT_ANTHROPIC_API_KEY",
        "model": "claude-3-5-sonnet-20241022",
        "timeout_ms": 25000,
        "max_attempts": 2,
        "max_tokens": 1024,
        "enabled": true
      }
    ]
  }
}
```

## 4. 密钥配置方式

推荐：环境变量

PowerShell:
```powershell
$env:ACTIONAGENT_OPENAI_API_KEY="sk-xxx"
$env:ACTIONAGENT_ANTHROPIC_API_KEY="sk-ant-xxx"
```

Bash:
```bash
export ACTIONAGENT_OPENAI_API_KEY="sk-xxx"
export ACTIONAGENT_ANTHROPIC_API_KEY="sk-ant-xxx"
```

说明：
1. 运行时不会把密钥写入日志。
2. 当 Provider 无可用密钥时，会跳过该 Provider。
3. 若所有 Provider 都无可用密钥，运行时会降级到本地静态适配器以保持可用性。

## 5. API 行为说明

1. `/v1/chat/completions`
   - 入参：OpenAI 风格 `model + messages`。
   - 出参：优先返回最小兼容 `chat.completion` 结构（含 `choices[0].message.content`）。

2. `/v1/responses`
   - 入参：`model + input`。
   - 出参：`response` 对象，包含 `output_text` 与运行时字段。

3. 路由与降级
   - 按 `primary -> fallbacks[]` 顺序选择 Provider。
   - 支持错误分类、重试退避、熔断与凭据冷却策略。

## 6. 快速验证

先启动 Agent，再执行：

```bash
curl -X POST http://127.0.0.1:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Say hello in one sentence."}]
  }'
```

仓库还提供验证脚本：
1. `scripts/verify-model-provider.ps1`
2. `scripts/verify-model-provider.sh`
