import type { BaseQueryApi } from "@reduxjs/toolkit/query";

import { clearSession, setSession } from "@/app/store/slices/authSlice";
import { httpBaseQuery } from "@/shared/api/base";

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
    role: "anonymous" | "viewer" | "admin";
    actor: string | null;
    sessionState: "anonymous" | "checking" | "authenticated";
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

describe("httpBaseQuery", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("refreshes and retries a protected request after a 401", async () => {
    const stateRef = {
      connection: {
        coreBaseUrl: "http://core.test"
      },
      auth: {
        accessToken: "expired-access",
        refreshToken: "refresh-1",
        role: "viewer" as const,
        actor: "alice",
        sessionState: "authenticated" as const
      }
    };
    const { api, dispatch } = createBaseQueryApi(stateRef);
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse(401, {
          error: {
            code: "unauthorized",
            message: "expired"
          }
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, {
          access_token: "fresh-access",
          refresh_token: "fresh-refresh"
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, {
          ok: true
        }),
      );

    const result = await httpBaseQuery(
      {
        url: "/v1/auth/me",
        method: "GET"
      },
      api,
      {},
    );

    expect(result).toEqual({
      data: {
        ok: true
      }
    });
    expect(stateRef.auth.accessToken).toBe("fresh-access");
    expect(stateRef.auth.refreshToken).toBe("fresh-refresh");
    expect(fetchSpy).toHaveBeenCalledTimes(3);
    expect(dispatch).toHaveBeenCalled();
  });

  it("clears the session when refresh fails", async () => {
    const stateRef = {
      connection: {
        coreBaseUrl: "http://core.test"
      },
      auth: {
        accessToken: "expired-access",
        refreshToken: "refresh-1",
        role: "admin" as const,
        actor: "root",
        sessionState: "authenticated" as const
      }
    };
    createBaseQueryApi(stateRef);
    const { api } = createBaseQueryApi(stateRef);

    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse(401, {
          error: {
            code: "unauthorized",
            message: "expired"
          }
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse(401, {
          error: {
            code: "unauthorized",
            message: "refresh failed"
          }
        }),
      );

    const result = await httpBaseQuery(
      {
        url: "/v1/auth/me",
        method: "GET"
      },
      api,
      {},
    );

    expect("error" in result && result.error?.code).toBe("unauthorized");
    expect(stateRef.auth.accessToken).toBeNull();
    expect(stateRef.auth.refreshToken).toBeNull();
    expect(stateRef.auth.role).toBe("anonymous");
  });
});
