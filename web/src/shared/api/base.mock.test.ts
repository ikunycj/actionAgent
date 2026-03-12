import type { BaseQueryApi } from "@reduxjs/toolkit/query";

import { clearSession, setSession } from "@/app/store/slices/authSlice";
import type { ApiMode, AuthRole, AuthSessionState, BridgeFrameRequest } from "@/shared/types/app";

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json"
    }
  });
}

function createBaseQueryApi(stateRef: {
  connection: {
    coreBaseUrl: string | null;
  };
  auth: {
    accessToken: string | null;
    refreshToken: string | null;
    role: AuthRole;
    actor: string | null;
    sessionState: AuthSessionState;
  };
}) {
  const dispatch = vi.fn((action: unknown) => {
    if (setSession.match(action)) {
      stateRef.auth = {
        ...stateRef.auth,
        ...action.payload,
        sessionState: "authenticated"
      };
    }

    if (clearSession.match(action)) {
      stateRef.auth = {
        accessToken: null,
        refreshToken: null,
        role: "anonymous",
        actor: null,
        sessionState: "anonymous"
      };
    }

    return action;
  });

  const api = {
    dispatch,
    getState: () => stateRef,
    signal: new AbortController().signal,
    abort: vi.fn(),
    endpoint: "test",
    type: "query",
    forced: false
  } as unknown as BaseQueryApi;

  return { api, dispatch };
}

type MockStateRef = Parameters<typeof createBaseQueryApi>[0];

async function loadBaseForMode(apiMode: ApiMode) {
  vi.resetModules();
  vi.doMock("@/shared/config/env", () => ({
    env: {
      appTitle: "ActionAgent WebUI",
      defaultCoreUrl: null,
      requestTimeoutMs: 15_000,
      apiMode,
      mockDelayMs: 0
    }
  }));

  const base = await import("./base");
  const runtime = await import("@/shared/mocks/runtime");
  runtime.resetMockRuntime();

  return {
    ...base,
    ...runtime
  };
}

describe("mock-capable base queries", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.resetModules();
    vi.doUnmock("@/shared/config/env");
  });

  it("serves health and auth from mock mode without calling fetch", async () => {
    const { directHttpBaseQuery, httpBaseQuery } = await loadBaseForMode("mock");
    const stateRef: MockStateRef = {
      connection: {
        coreBaseUrl: "http://core.test"
      },
      auth: {
        accessToken: null,
        refreshToken: null,
        role: "anonymous" as const,
        actor: null,
        sessionState: "anonymous" as const
      }
    };
    const { api } = createBaseQueryApi(stateRef);
    const fetchSpy = vi.spyOn(globalThis, "fetch");

    const health = await directHttpBaseQuery(
      {
        baseUrl: "http://core.test",
        request: {
          url: "/healthz",
          method: "GET"
        },
        includeAuth: false
      },
      api,
      {},
    );
    const login = await httpBaseQuery(
      {
        url: "/v1/auth/login",
        method: "POST",
        body: {
          mode: "token",
          token: "mock-admin-token"
        }
      },
      api,
      {},
    );

    expect(health).toEqual({
      data: {
        ok: true,
        ready: true,
        ts: "2026-03-10T10:00:00Z"
      }
    });
    expect("data" in login && (login.data as { role: string }).role).toBe("admin");
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("supports mock me after a mock login session is stored in state", async () => {
    const { httpBaseQuery } = await loadBaseForMode("mock");
    const stateRef: MockStateRef = {
      connection: {
        coreBaseUrl: "http://core.test"
      },
      auth: {
        accessToken: null,
        refreshToken: null,
        role: "anonymous" as const,
        actor: null,
        sessionState: "anonymous" as const
      }
    };
    const { api } = createBaseQueryApi(stateRef);
    const login = await httpBaseQuery(
      {
        url: "/v1/auth/login",
        method: "POST",
        body: {
          mode: "password",
          username: "viewer",
          password: "viewer123"
        }
      },
      api,
      {},
    );

    if (!("data" in login)) {
      throw new Error("expected mock login to succeed");
    }

    const session = login.data as {
      access_token: string;
      refresh_token: string;
      role: AuthRole;
      actor: string;
    };

    stateRef.auth = {
      ...stateRef.auth,
      accessToken: session.access_token,
      refreshToken: session.refresh_token,
      role: session.role,
      actor: session.actor,
      sessionState: "authenticated"
    };

    const me = await httpBaseQuery(
      {
        url: "/v1/auth/me",
        method: "GET"
      },
      api,
      {},
    );

    expect(me).toEqual({
      data: {
        role: "viewer",
        actor: "Viewer Operator"
      }
    });
  });

  it("falls back to mock bridge data in hybrid mode when the backend route is missing", async () => {
    const { bridgeBaseQuery } = await loadBaseForMode("hybrid");
    const stateRef: MockStateRef = {
      connection: {
        coreBaseUrl: "http://core.test"
      },
      auth: {
        accessToken: "live-token",
        refreshToken: "live-refresh",
        role: "admin" as const,
        actor: "alice",
        sessionState: "authenticated" as const
      }
    };
    const { api } = createBaseQueryApi(stateRef);
    const frame: BridgeFrameRequest = {
      type: "req",
      id: "task.list.2",
      method: "task.list",
      params: {
        limit: 2
      }
    };

    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse(404, {
        error: {
          code: "not_found",
          message: "missing"
        }
      }),
    );

    const result = await bridgeBaseQuery(frame, api, {});

    if (!("data" in result)) {
      throw new Error("expected hybrid bridge fallback to succeed");
    }

    const frameResponse = result.data as { payload: unknown };

    expect(frameResponse.payload).toEqual({
      tasks: expect.any(Array),
      count: 2
    });
  });

  it("does not replace real 401 auth failures with mock data in hybrid mode", async () => {
    const { httpBaseQuery } = await loadBaseForMode("hybrid");
    const stateRef: MockStateRef = {
      connection: {
        coreBaseUrl: "http://core.test"
      },
      auth: {
        accessToken: null,
        refreshToken: null,
        role: "anonymous" as const,
        actor: null,
        sessionState: "anonymous" as const
      }
    };
    const { api } = createBaseQueryApi(stateRef);

    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse(401, {
        error: {
          code: "unauthorized",
          message: "bad credentials"
        }
      }),
    );

    const result = await httpBaseQuery(
      {
        url: "/v1/auth/login",
        method: "POST",
        body: {
          mode: "token",
          token: "mock-admin-token"
        }
      },
      api,
      {},
    );

    expect("error" in result && result.error?.code).toBe("unauthorized");
  });
});
