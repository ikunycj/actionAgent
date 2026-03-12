import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { useAppDispatch, useAppSelector } from "@/app/store";
import { setActiveTaskId, setAutoRefresh } from "@/app/store/slices/tasksUiSlice";
import { useGetTaskQuery, useListTasksQuery } from "@/shared/api/coreBridgeApi";
import { getErrorMessage } from "@/shared/lib/errors";
import { useAgentConsoleScope } from "@/shared/hooks";
import {
  formatTaskSummary,
  formatTaskTimestamp,
  getTaskStateTone,
  isTerminalTaskState
} from "@/shared/lib/tasks";
import { Button, CodeBlock, StatusBadge } from "@/shared/ui";

const pollingIntervalMs = 5_000;

export function TasksScreen() {
  const dispatch = useAppDispatch();
  const scope = useAgentConsoleScope();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTaskId = useAppSelector((state) => state.tasksUi.activeTaskId);
  const listLimit = useAppSelector((state) => state.tasksUi.listLimit);
  const autoRefresh = useAppSelector((state) => state.tasksUi.autoRefresh);
  const {
    data: taskList,
    error: listError,
    isLoading: isListLoading,
    isFetching: isListFetching,
    refetch: refetchList
  } = useListTasksQuery(
    {
      agentId: scope.activeAgentId ?? "",
      limit: listLimit
    },
    {
      skip: !scope.activeAgentId
    },
  );

  const activeTask = useMemo(
    () => taskList?.tasks.find((task) => task.task_id === activeTaskId) ?? null,
    [activeTaskId, taskList?.tasks],
  );
  const [shouldPollDetail, setShouldPollDetail] = useState(false);
  const detailQuery = useGetTaskQuery(
    { agentId: scope.activeAgentId ?? "", taskId: activeTaskId ?? "" },
    {
      skip: !activeTaskId || !scope.activeAgentId,
      pollingInterval: shouldPollDetail ? pollingIntervalMs : 0,
      refetchOnMountOrArgChange: true,
      skipPollingIfUnfocused: true
    },
  );

  const tasks = taskList?.tasks ?? [];
  const taskDetail = detailQuery.data ?? activeTask;
  const routeTaskId = searchParams.get("taskId");

  useEffect(() => {
    if (!activeTaskId || !autoRefresh) {
      setShouldPollDetail(false);
      return;
    }

    if (!taskDetail) {
      setShouldPollDetail(true);
      return;
    }

    setShouldPollDetail(!isTerminalTaskState(taskDetail.state));
  }, [activeTaskId, autoRefresh, taskDetail]);

  useEffect(() => {
    if (routeTaskId && routeTaskId !== activeTaskId) {
      dispatch(setActiveTaskId(routeTaskId));
      return;
    }

    if (!routeTaskId && activeTaskId) {
      dispatch(setActiveTaskId(null));
    }
  }, [activeTaskId, dispatch, routeTaskId]);

  function openTask(taskId: string) {
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set("taskId", taskId);
    setSearchParams(nextParams, { replace: true });
    dispatch(setActiveTaskId(taskId));
  }

  function closeDrawer() {
    const nextParams = new URLSearchParams(searchParams);
    nextParams.delete("taskId");
    setSearchParams(nextParams, { replace: true });
    dispatch(setActiveTaskId(null));
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3 rounded-3xl border border-border bg-panel p-6 shadow-panel">
        <div className="space-y-2">
          <h1 className="text-xl font-semibold">Task Center</h1>
          <p className="max-w-2xl text-sm text-textMuted">
            Review recent task outcomes for agent `{scope.activeAgentId ?? "resolving"}`, inspect
            structured payloads, and keep an eye on running work without leaving the control plane.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge
            label={`Tasks: ${tasks.length}`}
            tone={tasks.length > 0 ? "success" : "neutral"}
          />
          <Button disabled={isListFetching} onClick={() => void refetchList()} tone="secondary">
            {isListFetching ? "Refreshing..." : "Refresh list"}
          </Button>
        </div>
      </div>

      <div className="overflow-hidden rounded-3xl border border-border bg-panel shadow-panel">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-border/80">
            <thead className="bg-panelAlt/60 text-left text-xs uppercase tracking-[0.16em] text-textMuted">
              <tr>
                <th className="px-4 py-3">Task</th>
                <th className="px-4 py-3">State</th>
                <th className="px-4 py-3">Started</th>
                <th className="px-4 py-3">Finished</th>
                <th className="px-4 py-3">Summary</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/60 text-sm text-text">
              {!scope.activeAgentId ? (
                <tr>
                  <td className="px-4 py-6 text-textMuted" colSpan={5}>
                    Resolving agent scope...
                  </td>
                </tr>
              ) : null}
              {isListLoading ? (
                <tr>
                  <td className="px-4 py-6 text-textMuted" colSpan={5}>
                    Loading recent tasks...
                  </td>
                </tr>
              ) : null}
              {!isListLoading && listError ? (
                <tr>
                  <td className="px-4 py-6 text-danger" colSpan={5}>
                    {getErrorMessage(listError)}
                  </td>
                </tr>
              ) : null}
              {!isListLoading && !listError && scope.activeAgentId && tasks.length === 0 ? (
                <tr>
                  <td className="px-4 py-6 text-textMuted" colSpan={5}>
                    No tasks available yet for this agent.
                  </td>
                </tr>
              ) : null}
              {tasks.map((task) => (
                <tr
                  key={task.task_id}
                  className={task.task_id === activeTaskId ? "bg-accentSoft/30" : "bg-transparent"}
                >
                  <td className="px-4 py-3 align-top">
                    <button
                      className="text-left"
                      onClick={() => openTask(task.task_id)}
                      type="button"
                    >
                      <span className="block font-medium text-text">{task.task_id}</span>
                      <span className="mt-1 block text-xs text-textMuted">{task.run_id}</span>
                    </button>
                  </td>
                  <td className="px-4 py-3 align-top">
                    <StatusBadge
                      label={task.state}
                      tone={getTaskStateTone(task.state)}
                    />
                  </td>
                  <td className="px-4 py-3 align-top text-textMuted">
                    {formatTaskTimestamp(task.started_at)}
                  </td>
                  <td className="px-4 py-3 align-top text-textMuted">
                    {formatTaskTimestamp(task.finished_at)}
                  </td>
                  <td className="px-4 py-3 align-top text-textMuted">
                    <p className="max-w-[28rem]">{formatTaskSummary(task)}</p>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {activeTaskId ? (
        <div className="fixed inset-0 z-40 flex justify-end bg-text/25 backdrop-blur-[2px]">
          <div className="flex h-full w-full max-w-2xl flex-col border-l border-border bg-panel shadow-panel">
            <div className="flex items-start justify-between gap-4 border-b border-border/80 px-6 py-5">
              <div className="space-y-2">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-textMuted">
                  Task Detail
                </p>
                <h2 className="text-lg font-semibold text-text">{activeTaskId}</h2>
                {taskDetail ? (
                  <StatusBadge
                    label={taskDetail.state}
                    tone={getTaskStateTone(taskDetail.state)}
                  />
                ) : null}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  disabled={!activeTaskId || detailQuery.isFetching}
                  onClick={() => void detailQuery.refetch()}
                  tone="secondary"
                >
                  {detailQuery.isFetching ? "Refreshing..." : "Refresh detail"}
                </Button>
                <Button
                  onClick={() => dispatch(setAutoRefresh(!autoRefresh))}
                  tone={autoRefresh ? "primary" : "secondary"}
                >
                  Auto refresh {autoRefresh ? "on" : "off"}
                </Button>
                <Button onClick={closeDrawer} tone="ghost">
                  Close
                </Button>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto px-6 py-5">
              {detailQuery.isLoading && !taskDetail ? (
                <p className="text-sm text-textMuted">Loading task detail...</p>
              ) : null}
              {detailQuery.error ? (
                <div className="rounded-2xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">
                  {getErrorMessage(detailQuery.error)}
                </div>
              ) : null}
              {taskDetail ? (
                <div className="space-y-6">
                  <section className="grid gap-4 md:grid-cols-2">
                    <article className="space-y-2 rounded-2xl border border-border/80 bg-panelAlt/40 p-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-textMuted">Agent</p>
                      <p className="font-medium text-text">{taskDetail.agent_id ?? "unknown"}</p>
                    </article>
                    <article className="space-y-2 rounded-2xl border border-border/80 bg-panelAlt/40 p-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-textMuted">Run</p>
                      <p className="font-medium text-text">{taskDetail.run_id}</p>
                    </article>
                    <article className="space-y-2 rounded-2xl border border-border/80 bg-panelAlt/40 p-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-textMuted">Node</p>
                      <p className="font-medium text-text">{taskDetail.node_id}</p>
                    </article>
                    <article className="space-y-2 rounded-2xl border border-border/80 bg-panelAlt/40 p-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-textMuted">Started</p>
                      <p className="font-medium text-text">
                        {formatTaskTimestamp(taskDetail.started_at)}
                      </p>
                    </article>
                    <article className="space-y-2 rounded-2xl border border-border/80 bg-panelAlt/40 p-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-textMuted">Finished</p>
                      <p className="font-medium text-text">
                        {formatTaskTimestamp(taskDetail.finished_at)}
                      </p>
                    </article>
                  </section>

                  <section className="space-y-3">
                    <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-textMuted">
                      Summary
                    </h3>
                    <div className="rounded-2xl border border-border/80 bg-panelAlt/40 p-4 text-sm text-text">
                      {formatTaskSummary(taskDetail)}
                    </div>
                  </section>

                  {taskDetail.error ? (
                    <section className="space-y-3">
                      <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-textMuted">
                        Error
                      </h3>
                      <div className="rounded-2xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">
                        {taskDetail.error}
                      </div>
                    </section>
                  ) : null}

                  <section className="space-y-3">
                    <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-textMuted">
                      Payload
                    </h3>
                    <CodeBlock>{JSON.stringify(taskDetail.payload, null, 2)}</CodeBlock>
                  </section>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
