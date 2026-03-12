export type AuthRole = "anonymous" | "viewer" | "admin";
export type AuthSessionState = "anonymous" | "checking" | "authenticated";
export type ConnectivityState = "unknown" | "checking" | "online" | "offline";
export type StatusTone = "neutral" | "success" | "warning" | "danger";
export type LoginMode = "token" | "password";
export type ApiMode = "live" | "mock" | "hybrid";

export type AppError = {
  code: string;
  message: string;
  status?: number;
  details?: unknown;
};

export type HealthStatus = {
  ok: boolean;
  ready: boolean;
  ts: string | null;
};

export type RuntimeAgent = {
  agent_id: string;
  is_default: boolean;
};

export type RuntimeAgentCatalog = {
  default_agent: string | null;
  agents: RuntimeAgent[];
  count: number;
};

export type AuthIdentity = {
  role: AuthRole;
  actor: string | null;
};

export type AuthSession = AuthIdentity & {
  accessToken: string | null;
  refreshToken: string | null;
};

export type TaskState = "QUEUED" | "RUNNING" | "SUCCEEDED" | "FAILED";

export type TaskOutcome = {
  task_id: string;
  run_id: string;
  agent_id: string | null;
  state: TaskState;
  node_id: string;
  error: string;
  replay: boolean;
  payload: Record<string, unknown>;
  started_at: string;
  finished_at: string | null;
};

export type TaskListPayload = {
  tasks: TaskOutcome[];
  count: number;
};

export type LoginCredentials =
  | {
      mode: "token";
      token: string;
    }
  | {
      mode: "password";
      username: string;
      password: string;
    };

export type BridgeFrameRequest = {
  type: "req";
  id: string;
  method: string;
  params?: unknown;
  session_id?: string;
};

export type BridgeFrameResponse = {
  type: "res";
  id: string;
  ok: boolean;
  payload?: unknown;
  error?: {
    code: string;
    message: string;
    details?: unknown;
  };
};
