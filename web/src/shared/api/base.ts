import {
  type BaseQueryApi,
  type BaseQueryFn,
  type FetchArgs,
  type FetchBaseQueryError,
  fetchBaseQuery
} from "@reduxjs/toolkit/query";

import type { RootState } from "@/app/store";
import { clearSession, setSession } from "@/app/store/slices/authSlice";
import { buildRefreshRequest, mergeAuthSession, normalizeAuthSession } from "@/shared/api/auth";
import { env } from "@/shared/config/env";
import { normalizeCoreBaseUrl } from "@/shared/lib/coreUrl";
import {
  executeMockBridgeRequest,
  executeMockHttpRequest,
  shouldFallbackToMock
} from "@/shared/mocks/runtime";
import type { AppError, BridgeFrameRequest, BridgeFrameResponse } from "@/shared/types/app";

function getRuntimeBaseUrl(api: Pick<BaseQueryApi, "getState">) {
  const state = api.getState() as RootState;
  return normalizeCoreBaseUrl(state.connection.coreBaseUrl ?? env.defaultCoreUrl);
}

function getRawBaseQuery(baseUrl: string, includeAuth = true) {
  return fetchBaseQuery({
    baseUrl,
    timeout: env.requestTimeoutMs,
    prepareHeaders: (headers, api) => {
      const state = api.getState() as RootState;
      if (includeAuth && state.auth.accessToken) {
        headers.set("Authorization", `Bearer ${state.auth.accessToken}`);
      }
      headers.set("Content-Type", "application/json");
      return headers;
    }
  });
}

export function toAppError(error: unknown, fallbackMessage: string): AppError {
  if (error && typeof error === "object" && "status" in error) {
    const result = error as FetchBaseQueryError;

    if (typeof result.status === "string") {
      return {
        code: result.status.toLowerCase(),
        message: "error" in result && typeof result.error === "string" ? result.error : fallbackMessage
      };
    }

    const payload =
      result.data && typeof result.data === "object" && "error" in result.data
        ? (result.data as { error?: { code?: string; message?: string; details?: unknown } })
            .error
        : undefined;

    return {
      code: payload?.code ?? "request_failed",
      message: payload?.message ?? fallbackMessage,
      status: typeof result.status === "number" ? result.status : undefined,
      details: payload?.details ?? result.data
    };
  }

  if (error instanceof Error) {
    return {
      code: "runtime_error",
      message: error.message
    };
  }

  return {
    code: "unknown_error",
    message: fallbackMessage
  };
}

type RawHttpResult = Awaited<ReturnType<ReturnType<typeof fetchBaseQuery>>>;
type TransportResult = RawHttpResult;

async function executeRawRequest(
  baseUrl: string,
  args: string | FetchArgs,
  api: BaseQueryApi,
  extraOptions: object,
  includeAuth = true,
): Promise<RawHttpResult> {
  const rawBaseQuery = getRawBaseQuery(baseUrl, includeAuth);

  return rawBaseQuery(args, api, extraOptions);
}

async function executeHttpTransport(
  baseUrl: string,
  args: string | FetchArgs,
  api: BaseQueryApi,
  extraOptions: object,
  includeAuth = true,
): Promise<TransportResult> {
  if (env.apiMode === "mock") {
    const state = api.getState() as RootState;

    return executeMockHttpRequest({
      request: typeof args === "string" ? { url: args } : args,
      context: {
        accessToken: includeAuth ? state.auth.accessToken : null,
        refreshToken: state.auth.refreshToken
      },
      delayMs: env.mockDelayMs
    }) as Promise<TransportResult>;
  }

  const liveResult = await executeRawRequest(baseUrl, args, api, extraOptions, includeAuth);

  if (liveResult.error) {
    const error = toAppError(liveResult.error, "Core request failed.");

    if (shouldFallbackToMock(env.apiMode, error)) {
      const state = api.getState() as RootState;

      return executeMockHttpRequest({
        request: typeof args === "string" ? { url: args } : args,
        context: {
          accessToken: includeAuth ? state.auth.accessToken : null,
          refreshToken: state.auth.refreshToken
        },
        delayMs: env.mockDelayMs
      }) as Promise<TransportResult>;
    }
  }

  return liveResult;
}

function getRequestUrl(args: string | FetchArgs) {
  return typeof args === "string" ? args : args.url;
}

function shouldSkipRefresh(args: string | FetchArgs) {
  const url = getRequestUrl(args);

  return url.startsWith("/v1/auth/login") || url.startsWith("/v1/auth/refresh");
}

function isUnauthorized(result: RawHttpResult) {
  return Boolean(result.error && typeof result.error.status === "number" && result.error.status === 401);
}

let refreshPromise: Promise<boolean> | null = null;

async function refreshSession(
  baseUrl: string,
  api: BaseQueryApi,
  extraOptions: object,
) {
  const state = api.getState() as RootState;
  const existingSession = state.auth;
  const refreshToken = existingSession.refreshToken;

  if (!refreshToken) {
    api.dispatch(clearSession());
    return false;
  }

  if (!refreshPromise) {
    refreshPromise = (async () => {
      const result = await executeRawRequest(
        baseUrl,
        {
          url: "/v1/auth/refresh",
          method: "POST",
          body: buildRefreshRequest(refreshToken)
        },
        api,
        extraOptions,
        false,
      );

      if (result.error) {
        api.dispatch(clearSession());
        return false;
      }

      const mergedSession = mergeAuthSession(existingSession, normalizeAuthSession(result.data));

      if (!mergedSession.accessToken) {
        api.dispatch(clearSession());
        return false;
      }

      api.dispatch(setSession(mergedSession));
      return true;
    })().finally(() => {
      refreshPromise = null;
    });
  }

  return refreshPromise;
}

export async function directHttpBaseQuery(
  params: {
    baseUrl: string;
    request: string | FetchArgs;
    includeAuth?: boolean;
  },
  api: BaseQueryApi,
  extraOptions: object,
): Promise<{ data: unknown } | { error: AppError }> {
  const normalizedBaseUrl = normalizeCoreBaseUrl(params.baseUrl);

  if (!normalizedBaseUrl) {
    return {
      error: {
        code: "core_url_missing",
        message: "Core base URL is not configured."
      }
    };
  }

  const result = await executeHttpTransport(
    normalizedBaseUrl,
    params.request,
    api,
    extraOptions,
    params.includeAuth ?? true,
  );

  if (result.error) {
    return {
      error: toAppError(result.error, "Core request failed.")
    };
  }

  return { data: result.data };
}

export const httpBaseQuery: BaseQueryFn<string | FetchArgs, unknown, AppError> = async (
  args,
  api,
  extraOptions,
) => {
  const baseUrl = getRuntimeBaseUrl(api);

  if (!baseUrl) {
    return {
      error: {
        code: "core_url_missing",
        message: "Core base URL is not configured."
      }
    };
  }

  let result = await executeHttpTransport(baseUrl, args, api, extraOptions, true);

  if (isUnauthorized(result) && !shouldSkipRefresh(args)) {
    const refreshed = await refreshSession(baseUrl, api, extraOptions);

    if (refreshed) {
      result = await executeHttpTransport(baseUrl, args, api, extraOptions, true);
    } else {
      return {
        error: {
          code: "unauthorized",
          message: "Authentication expired. Please log in again.",
          status: 401
        }
      };
    }
  }

  if (result.error) {
    return {
      error: toAppError(result.error, "Core request failed.")
    };
  }

  return { data: result.data };
};

export const bridgeBaseQuery: BaseQueryFn<BridgeFrameRequest, BridgeFrameResponse, AppError> =
  async (args, api, extraOptions) => {
    const baseUrl = getRuntimeBaseUrl(api);

    if (!baseUrl) {
      return {
        error: {
          code: "core_url_missing",
          message: "Core base URL is not configured."
        }
      };
    }

    let frame: BridgeFrameResponse;

    if (env.apiMode === "mock") {
      frame = await executeMockBridgeRequest({
        frame: args,
        delayMs: env.mockDelayMs
      });
    } else {
      const liveResult = await executeRawRequest(
        baseUrl,
        {
          url: "/ws/frame",
          method: "POST",
          body: args
        },
        api,
        extraOptions,
        true,
      );

      if (liveResult.error) {
        const error = toAppError(liveResult.error, "Core bridge request failed.");

        if (shouldFallbackToMock(env.apiMode, error)) {
          frame = await executeMockBridgeRequest({
            frame: args,
            delayMs: env.mockDelayMs
          });
        } else {
          return {
            error
          };
        }
      } else {
        frame = liveResult.data as BridgeFrameResponse;
      }
    }

    if (!frame.ok) {
      return {
        error: {
          code: frame.error?.code ?? "bridge_request_failed",
          message: frame.error?.message ?? "Core bridge request failed.",
          details: frame.error?.details
        }
      };
    }

    return {
      data: frame
    };
  };
