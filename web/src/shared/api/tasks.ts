import type { BridgeFrameResponse, TaskListPayload, TaskOutcome } from "@/shared/types/app";

function asRecord(value: unknown) {
  if (!value || typeof value !== "object") {
    return null;
  }

  return value as Record<string, unknown>;
}

function asTaskOutcome(value: unknown): TaskOutcome {
  const record = asRecord(value);

  return {
    task_id: typeof record?.task_id === "string" ? record.task_id : "",
    run_id: typeof record?.run_id === "string" ? record.run_id : "",
    agent_id:
      typeof record?.agent_id === "string"
        ? record.agent_id
        : typeof record?.payload === "object" &&
            record.payload &&
            typeof (record.payload as Record<string, unknown>).agent_id === "string"
          ? ((record.payload as Record<string, unknown>).agent_id as string)
          : null,
    state:
      record?.state === "QUEUED" ||
      record?.state === "RUNNING" ||
      record?.state === "SUCCEEDED" ||
      record?.state === "FAILED"
        ? record.state
        : "FAILED",
    node_id: typeof record?.node_id === "string" ? record.node_id : "unknown",
    error: typeof record?.error === "string" ? record.error : "",
    replay: Boolean(record?.replay),
    payload: record?.payload && typeof record.payload === "object" ? (record.payload as Record<string, unknown>) : {},
    started_at: typeof record?.started_at === "string" ? record.started_at : "",
    finished_at: typeof record?.finished_at === "string" ? record.finished_at : null
  };
}

export function normalizeTaskListResponse(frame: BridgeFrameResponse): TaskListPayload {
  const payload = asRecord(frame.payload);
  const tasks = Array.isArray(payload?.tasks) ? payload.tasks.map(asTaskOutcome) : [];
  const count = typeof payload?.count === "number" ? payload.count : tasks.length;

  return {
    tasks,
    count
  };
}

export function normalizeTaskDetailResponse(frame: BridgeFrameResponse): TaskOutcome {
  const payload = asRecord(frame.payload);
  return asTaskOutcome(payload?.task);
}
