import type { StatusTone, TaskOutcome, TaskState } from "@/shared/types/app";

const terminalStates: TaskState[] = ["SUCCEEDED", "FAILED"];

export function isTerminalTaskState(state: TaskState | null | undefined) {
  return Boolean(state && terminalStates.includes(state));
}

export function getTaskStateTone(state: TaskState): StatusTone {
  switch (state) {
    case "SUCCEEDED":
      return "success";
    case "FAILED":
      return "danger";
    case "RUNNING":
      return "warning";
    case "QUEUED":
    default:
      return "neutral";
  }
}

export function formatTaskSummary(task: TaskOutcome) {
  const outputText =
    task.payload.output &&
    typeof task.payload.output === "object" &&
    "text" in task.payload.output &&
    typeof task.payload.output.text === "string"
      ? task.payload.output.text
      : null;

  if (outputText?.trim()) {
    return outputText.trim();
  }

  if (task.error.trim()) {
    return task.error.trim();
  }

  return "No summary available.";
}

export function formatTaskTimestamp(value: string | null) {
  if (!value) {
    return "In progress";
  }

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(date);
}
