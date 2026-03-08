# ActionAgent Core Agent API

## 1. 约定
- Base URL：`http://127.0.0.1:8000`（默认）
- Content-Type：`application/json`
- Agent 选择优先级：`body.agent_id` > `X-Agent-ID` > `default_agent`
- 幂等键：`idempotency_key`（可选）

错误响应格式（HTTP）：

```json
{
  "error": {
    "code": "validation_error",
    "message": "input is required"
  }
}
```

## 2. HTTP 接口

### 2.1 GET /healthz
用途：健康检查与就绪状态。  
返回：

```json
{
  "ok": true,
  "ready": true,
  "ts": "2026-03-08T15:04:05Z"
}
```

### 2.2 POST /v1/run
用途：提交通用任务执行。

请求体：

```json
{
  "agent_id": "default",
  "session_key": "s1",
  "idempotency_key": "run-001",
  "input": {
    "text": "Summarize this paragraph."
  }
}
```

成功响应（`task.Outcome`）：

```json
{
  "task_id": "task-...",
  "run_id": "run-...",
  "state": "SUCCEEDED",
  "node_id": "local",
  "error": "",
  "replay": false,
  "payload": {
    "agent_id": "default",
    "provider": "openai-main",
    "model": "gpt-4o-mini",
    "output": {
      "text": "..."
    }
  },
  "started_at": "2026-03-08T15:04:05Z",
  "finished_at": "2026-03-08T15:04:06Z"
}
```

### 2.3 POST /v1/chat/completions
用途：OpenAI Chat Completions 兼容入口。

请求体：

```json
{
  "agent_id": "default",
  "model": "gpt-4o-mini",
  "idempotency_key": "chat-001",
  "messages": [
    {
      "role": "user",
      "content": "Say hello in one sentence."
    }
  ]
}
```

成功响应（兼容模式）：

```json
{
  "id": "task-...",
  "object": "chat.completion",
  "created": 1770000000,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "finish_reason": "stop",
      "message": {
        "role": "assistant",
        "content": "..."
      }
    }
  ],
  "run_id": "run-...",
  "agent_id": "default",
  "state": "SUCCEEDED",
  "replay": false,
  "payload": {}
}
```

### 2.4 POST /v1/responses
用途：OpenAI Responses 风格入口。

请求体（非流式）：

```json
{
  "agent_id": "default",
  "model": "gpt-4o-mini",
  "input": [
    {
      "role": "user",
      "content": "Write one sentence."
    }
  ],
  "stream": false
}
```

成功响应：

```json
{
  "id": "task-...",
  "object": "response",
  "status": "completed",
  "model": "gpt-4o-mini",
  "agent_id": "default",
  "output_text": "...",
  "run_id": "run-...",
  "state": "SUCCEEDED",
  "replay": false,
  "payload": {}
}
```

流式模式：`"stream": true`  
行为：网关透传上游 `text/event-stream`（支持 `/responses` stream）。

### 2.5 GET /events
用途：订阅实时事件流（JSON 行流，非 SSE）。  
返回帧格式（`WSFrame{type=event}`）：

```json
{
  "type": "event",
  "event": "request.finished",
  "payload": {
    "state": "SUCCEEDED"
  },
  "connection_id": "",
  "session_id": "s1",
  "seq": 2
}
```

### 2.6 GET /metrics
用途：获取运行指标快照。  
示例字段：
- `task_success`
- `task_fail`
- `queue_depth`
- `active_concurrent`
- `node_online`
- `model_agent_<id>_route_ok`

### 2.7 GET /alerts
用途：获取告警评估结果。  
响应：

```json
{
  "alerts": [
    {
      "code": "queue_depth_high",
      "severity": "warn",
      "message": "queue depth exceeded warning threshold",
      "value": 20,
      "threshold": 8
    }
  ],
  "count": 1
}
```

## 3. WS Frame Bridge 接口

### 3.1 POST /ws/frame
请求格式：

```json
{
  "type": "req",
  "id": "req-1",
  "method": "agent.run",
  "params": {
    "agent_id": "default",
    "x": 1
  },
  "connection_id": "conn-1",
  "session_id": "s1"
}
```

响应格式：

```json
{
  "type": "res",
  "id": "req-1",
  "ok": true,
  "payload": {}
}
```

### 3.2 支持方法
1. `agent.run`
2. `agent.wait`（参数：`task_id`、`timeout_ms`）
3. `task.get`
4. `task.list`（参数：`limit`）
5. `audit.query`（参数：`limit`、`tool`、`decision`）
6. `approval.list`（参数：`limit`）
7. `session.stats`
8. `session.maintain`
9. `observability.alerts`

未支持方法返回：

```json
{
  "type": "res",
  "id": "req-1",
  "ok": false,
  "error": "unsupported method: xxx"
}
```

## 4. 常见错误码
1. `method_not_allowed`：HTTP 方法错误。
2. `invalid_json`：请求体 JSON 解析失败。
3. `validation_error`：参数不完整或 `agent_id` 非法。
4. `execution_failed`：执行链路失败（模型、调度、运行时异常）。
5. `not_ready`：runtime 尚未 ready 或执行器不可用。

## 5. curl 快速验证

```bash
curl -X POST http://127.0.0.1:8000/v1/run \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"default","input":{"text":"hello"}}'
```

```bash
curl -X POST http://127.0.0.1:8000/ws/frame \
  -H "Content-Type: application/json" \
  -d '{"type":"req","id":"1","method":"task.list","params":{"limit":10}}'
```
