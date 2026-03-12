import type {
  ApiMode,
  AppError,
  AuthRole,
  BridgeFrameRequest,
  BridgeFrameResponse,
  TaskOutcome
} from "@/shared/types/app";

type MockTransportError = {
  status: number | string;
  data?: {
    error: AppError;
  };
  error?: string;
};

type MockTransportResult = { data: unknown } | { error: MockTransportError };

type MockIdentity = {
  role: Exclude<AuthRole, "anonymous">;
  actor: string;
};

type MockContext = {
  accessToken: string | null;
  refreshToken: string | null;
};

const mockUsers = {
  admin: {
    password: "admin123",
    role: "admin" as const,
    actor: "Admin Operator"
  },
  viewer: {
    password: "viewer123",
    role: "viewer" as const,
    actor: "Viewer Operator"
  }
} as const;

const mockTasks: TaskOutcome[] = [
  {
    task_id: "task-mock-1001",
    run_id: "run-mock-1001",
    agent_id: "default",
    state: "SUCCEEDED",
    node_id: "mock-local",
    error: "",
    replay: false,
    payload: {
      agent_id: "default",
      provider: "mock-openai",
      model: "gpt-4o-mini",
      output: {
        text: "Generated overview summary from mock transport."
      }
    },
    started_at: "2026-03-10T10:20:00Z",
    finished_at: "2026-03-10T10:20:02Z"
  },
  {
    task_id: "task-mock-1002",
    run_id: "run-mock-1002",
    agent_id: "default",
    state: "FAILED",
    node_id: "mock-local",
    error: "Mock upstream timed out while waiting for approval.",
    replay: false,
    payload: {
      agent_id: "default",
      provider: "mock-openai",
      model: "gpt-4o-mini",
      output: {
        text: ""
      }
    },
    started_at: "2026-03-10T10:23:00Z",
    finished_at: "2026-03-10T10:23:09Z"
  },
  {
    task_id: "task-mock-1003",
    run_id: "run-mock-1003",
    agent_id: "default",
    state: "RUNNING",
    node_id: "mock-local",
    error: "",
    replay: false,
    payload: {
      agent_id: "default",
      provider: "mock-openai",
      model: "gpt-4o-mini",
      output: {
        text: "Streaming mock response..."
      }
    },
    started_at: "2026-03-10T10:25:00Z",
    finished_at: null
  }
];

let sessionCounter = 0;
const revokedAccessTokens = new Set<string>();
const revokedRefreshTokens = new Set<string>();

function delay(ms: number) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

function success(data: unknown): MockTransportResult {
  return { data };
}

function failure(status: number | string, code: string, message: string, details?: unknown) {
  return {
    error: {
      status,
      data: {
        error: {
          code,
          message,
          details
        }
      }
    }
  } satisfies MockTransportResult;
}

function buildToken(kind: "access" | "refresh", role: MockIdentity["role"]) {
  sessionCounter += 1;
  return `mock-${kind}-${role}-${sessionCounter}`;
}

function resolveIdentityFromToken(token: string | null): MockIdentity | null {
  if (!token) {
    return null;
  }

  if (revokedAccessTokens.has(token) || revokedRefreshTokens.has(token)) {
    return null;
  }

  const match = token.match(/^mock-(?:access|refresh)-(admin|viewer)-\d+$/);

  if (!match) {
    return null;
  }

  const role = match[1] as MockIdentity["role"];
  return {
    role,
    actor: role === "admin" ? mockUsers.admin.actor : mockUsers.viewer.actor
  };
}

function issueSession(identity: MockIdentity) {
  return {
    access_token: buildToken("access", identity.role),
    refresh_token: buildToken("refresh", identity.role),
    role: identity.role,
    actor: identity.actor
  };
}

function resolveIdentityFromLoginBody(body: unknown): MockIdentity | null {
  if (!body || typeof body !== "object") {
    return null;
  }

  const record = body as Record<string, unknown>;

  if (record.mode === "token" && typeof record.token === "string") {
    const token = record.token.toLowerCase();

    if (token.includes("admin")) {
      return {
        role: "admin",
        actor: mockUsers.admin.actor
      };
    }

    if (token.includes("viewer")) {
      return {
        role: "viewer",
        actor: mockUsers.viewer.actor
      };
    }
  }

  if (
    record.mode === "password" &&
    typeof record.username === "string" &&
    typeof record.password === "string"
  ) {
    const username = record.username.toLowerCase();
    const user = username === "admin" ? mockUsers.admin : username === "viewer" ? mockUsers.viewer : null;

    if (user && user.password === record.password) {
      return {
        role: user.role,
        actor: user.actor
      };
    }
  }

  return null;
}

function pickTaskById(taskId: string | null) {
  if (!taskId) {
    return null;
  }

  return mockTasks.find((task) => task.task_id === taskId) ?? null;
}

function getMethod(method: string | undefined) {
  return method?.toUpperCase() ?? "GET";
}

export function resetMockRuntime() {
  sessionCounter = 0;
  revokedAccessTokens.clear();
  revokedRefreshTokens.clear();
}

export async function executeMockHttpRequest(params: {
  request: {
    url: string;
    method?: string;
    body?: unknown;
  };
  context: MockContext;
  delayMs: number;
}): Promise<MockTransportResult> {
  await delay(params.delayMs);

  const method = getMethod(params.request.method);
  const key = `${method} ${params.request.url}`;

  switch (key) {
    case "GET /healthz":
      return success({
        ok: true,
        ready: true,
        ts: "2026-03-10T10:00:00Z"
      });

    case "POST /v1/auth/login": {
      const identity = resolveIdentityFromLoginBody(params.request.body);

      if (!identity) {
        return failure(
          401,
          "invalid_credentials",
          "Mock login failed. Use mock-admin-token, mock-viewer-token, admin/admin123, or viewer/viewer123.",
        );
      }

      return success(issueSession(identity));
    }

    case "POST /v1/auth/refresh": {
      const body = (params.request.body ?? {}) as Record<string, unknown>;
      const refreshToken = typeof body.refresh_token === "string" ? body.refresh_token : null;
      const identity = resolveIdentityFromToken(refreshToken);

      if (!identity) {
        return failure(401, "unauthorized", "Mock refresh token is invalid or expired.");
      }

      return success(issueSession(identity));
    }

    case "POST /v1/auth/logout": {
      const body = (params.request.body ?? {}) as Record<string, unknown>;
      const refreshToken = typeof body.refresh_token === "string" ? body.refresh_token : params.context.refreshToken;

      if (refreshToken) {
        revokedRefreshTokens.add(refreshToken);
      }

      if (params.context.accessToken) {
        revokedAccessTokens.add(params.context.accessToken);
      }

      return success({
        ok: true
      });
    }

    case "GET /v1/auth/me": {
      const identity = resolveIdentityFromToken(params.context.accessToken);

      if (!identity) {
        return failure(401, "unauthorized", "Mock access token is invalid or expired.");
      }

      return success({
        role: identity.role,
        actor: identity.actor
      });
    }

    case "GET /v1/runtime/agents":
      return success({
        default_agent: "default",
        agents: [
          {
            agent_id: "default",
            is_default: true
          }
        ],
        count: 1
      });

    default:
      return failure(501, "mock_not_implemented", `Mock handler not implemented for ${key}.`);
  }
}

export async function executeMockBridgeRequest(params: {
  frame: BridgeFrameRequest;
  delayMs: number;
}): Promise<BridgeFrameResponse> {
  await delay(params.delayMs);

  switch (params.frame.method) {
    case "task.list": {
      const record = (params.frame.params ?? {}) as Record<string, unknown>;
      const agentId = typeof record.agent_id === "string" ? record.agent_id : null;
      const limit = typeof record.limit === "number" ? record.limit : 20;
      const visibleTasks = agentId
        ? mockTasks.filter((task) => task.agent_id === agentId)
        : mockTasks;

      return {
        type: "res",
        id: params.frame.id,
        ok: true,
        payload: {
          tasks: visibleTasks.slice(0, limit),
          count: Math.min(limit, visibleTasks.length)
        }
      };
    }

    case "task.get": {
      const record = (params.frame.params ?? {}) as Record<string, unknown>;
      const agentId = typeof record.agent_id === "string" ? record.agent_id : null;
      const taskId =
        typeof record.task_id === "string"
          ? record.task_id
          : typeof record.taskId === "string"
            ? record.taskId
            : null;
      const task = pickTaskById(taskId);

      if (!task || (agentId && task.agent_id !== agentId)) {
        return {
          type: "res",
          id: params.frame.id,
          ok: false,
          error: {
            code: "not_found",
            message: "Mock task was not found."
          }
        };
      }

      return {
        type: "res",
        id: params.frame.id,
        ok: true,
        payload: {
          task
        }
      };
    }

    default:
      return {
        type: "res",
        id: params.frame.id,
        ok: false,
        error: {
          code: "method_not_supported",
          message: `Mock bridge method not implemented: ${params.frame.method}`
        }
      };
  }
}

export function shouldFallbackToMock(mode: ApiMode, error: AppError) {
  if (mode !== "hybrid") {
    return false;
  }

  if (error.code === "fetch_error" || error.code === "timeout_error") {
    return true;
  }

  return Boolean(error.status && [404, 405, 501, 503].includes(error.status));
}
