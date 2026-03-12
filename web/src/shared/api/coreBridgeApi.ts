import { createApi } from "@reduxjs/toolkit/query/react";

import { bridgeBaseQuery } from "@/shared/api/base";
import { normalizeTaskDetailResponse, normalizeTaskListResponse } from "@/shared/api/tasks";
import type { BridgeFrameResponse, TaskListPayload, TaskOutcome } from "@/shared/types/app";

export const coreBridgeApi = createApi({
  reducerPath: "coreBridgeApi",
  baseQuery: bridgeBaseQuery,
  tagTypes: ["Tasks"],
  endpoints: (builder) => ({
    listTasks: builder.query<TaskListPayload, { agentId: string; limit?: number }>({
      query: (params) => ({
        type: "req",
        id: `task.list.${params.agentId}.${params.limit ?? 20}`,
        method: "task.list",
        params: {
          agent_id: params.agentId,
          limit: params.limit ?? 20
        }
      }),
      providesTags: ["Tasks"],
      transformResponse: (frame: BridgeFrameResponse) => normalizeTaskListResponse(frame)
    }),
    getTask: builder.query<TaskOutcome, { agentId: string; taskId: string }>({
      query: ({ agentId, taskId }) => ({
        type: "req",
        id: `task.get.${agentId}.${taskId}`,
        method: "task.get",
        params: {
          agent_id: agentId,
          task_id: taskId
        }
      }),
      providesTags: (_result, _error, arg) => [{ type: "Tasks", id: arg.taskId }],
      transformResponse: (frame: BridgeFrameResponse) => normalizeTaskDetailResponse(frame)
    })
  })
});

export const { useGetTaskQuery, useListTasksQuery } = coreBridgeApi;
