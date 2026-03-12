import { createApi } from "@reduxjs/toolkit/query/react";

import { clearSession, finishSessionCheck, setSession, setSessionState, updateIdentity } from "@/app/store/slices/authSlice";
import { setConnectivityState, setLastConnectedAt } from "@/app/store/slices/connectionSlice";
import {
  buildLoginRequest,
  buildLogoutRequest,
  normalizeAuthIdentity,
  normalizeAuthSession
} from "@/shared/api/auth";
import { directHttpBaseQuery, httpBaseQuery } from "@/shared/api/base";
import type {
  AppError,
  HealthStatus,
  LoginCredentials,
  RuntimeAgentCatalog
} from "@/shared/types/app";

type UnknownRecord = Record<string, unknown>;

function asRecord(value: unknown): UnknownRecord | null {
  if (!value || typeof value !== "object") {
    return null;
  }

  return value as UnknownRecord;
}

function normalizeHealthStatus(payload: unknown): HealthStatus {
  const record = asRecord(payload);

  return {
    ok: Boolean(record?.ok),
    ready: Boolean(record?.ready),
    ts: typeof record?.ts === "string" ? record.ts : null
  };
}

function normalizeRuntimeAgentCatalog(payload: unknown): RuntimeAgentCatalog {
  const record = asRecord(payload);
  const rawAgents = Array.isArray(record?.agents) ? record.agents : [];

  return {
    default_agent: typeof record?.default_agent === "string" ? record.default_agent : null,
    agents: rawAgents
      .map((item) => asRecord(item))
      .filter((item): item is UnknownRecord => item !== null)
      .map((item) => ({
        agent_id: typeof item.agent_id === "string" ? item.agent_id : "",
        is_default: Boolean(item.is_default)
      }))
      .filter((item) => item.agent_id.length > 0),
    count: typeof record?.count === "number" ? record.count : rawAgents.length
  };
}

export const coreHttpApi = createApi({
  reducerPath: "coreHttpApi",
  baseQuery: httpBaseQuery,
  tagTypes: ["Health", "Auth", "Config", "Sessions", "Transcript", "Agents"],
  endpoints: (builder) => ({
    getHealth: builder.query<HealthStatus, void>({
      query: () => ({
        url: "/healthz",
        method: "GET"
      }),
      providesTags: ["Health"],
      transformResponse: normalizeHealthStatus,
      async onQueryStarted(_, { dispatch, queryFulfilled }) {
        dispatch(setConnectivityState("checking"));

        try {
          const { data } = await queryFulfilled;
          dispatch(setConnectivityState(data.ready ? "online" : "offline"));

          if (data.ready) {
            dispatch(setLastConnectedAt(data.ts ?? new Date().toISOString()));
          }
        } catch {
          dispatch(setConnectivityState("offline"));
        }
      }
    }),
    checkHealth: builder.mutation<HealthStatus, { baseUrl: string }>({
      queryFn: async ({ baseUrl }, api, extraOptions) => {
        const result = await directHttpBaseQuery(
          {
            baseUrl,
            request: {
              url: "/healthz",
              method: "GET"
            },
            includeAuth: false
          },
          api,
          extraOptions,
        );

        if ("error" in result) {
          return {
            error: result.error as AppError
          };
        }

        return {
          data: normalizeHealthStatus(result.data)
        };
      }
    }),
    getRuntimeAgents: builder.query<RuntimeAgentCatalog, void>({
      query: () => ({
        url: "/v1/runtime/agents",
        method: "GET"
      }),
      providesTags: ["Agents"],
      transformResponse: normalizeRuntimeAgentCatalog
    }),
    login: builder.mutation<ReturnType<typeof normalizeAuthSession>, LoginCredentials>({
      query: (credentials) => ({
        url: "/v1/auth/login",
        method: "POST",
        body: buildLoginRequest(credentials)
      }),
      invalidatesTags: ["Auth"],
      transformResponse: normalizeAuthSession,
      async onQueryStarted(_, { dispatch, queryFulfilled }) {
        const { data } = await queryFulfilled;
        dispatch(setSession(data));
      }
    }),
    refresh: builder.mutation<ReturnType<typeof normalizeAuthSession>, { refreshToken: string }>({
      query: ({ refreshToken }) => ({
        url: "/v1/auth/refresh",
        method: "POST",
        body: {
          refresh_token: refreshToken
        }
      }),
      invalidatesTags: ["Auth"],
      transformResponse: normalizeAuthSession,
      async onQueryStarted(_, { dispatch, queryFulfilled }) {
        const { data } = await queryFulfilled;
        dispatch(setSession(data));
      }
    }),
    logout: builder.mutation<{ ok: boolean }, { refreshToken: string | null }>({
      query: ({ refreshToken }) => ({
        url: "/v1/auth/logout",
        method: "POST",
        body: buildLogoutRequest(refreshToken)
      }),
      invalidatesTags: ["Auth"],
      async onQueryStarted(_, { dispatch, queryFulfilled }) {
        try {
          await queryFulfilled;
        } finally {
          dispatch(clearSession());
        }
      }
    }),
    getMe: builder.query<ReturnType<typeof normalizeAuthIdentity>, void>({
      query: () => ({
        url: "/v1/auth/me",
        method: "GET"
      }),
      providesTags: ["Auth"],
      transformResponse: normalizeAuthIdentity,
      async onQueryStarted(_, { dispatch, queryFulfilled }) {
        dispatch(setSessionState("checking"));

        try {
          const { data } = await queryFulfilled;
          dispatch(updateIdentity(data));
        } finally {
          dispatch(finishSessionCheck());
        }
      }
    })
  })
});

export const {
  useCheckHealthMutation,
  useGetHealthQuery,
  useGetRuntimeAgentsQuery,
  useGetMeQuery,
  useLoginMutation,
  useLogoutMutation,
  useRefreshMutation
} = coreHttpApi;
