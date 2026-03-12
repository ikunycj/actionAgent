import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Provider } from "react-redux";
import { MemoryRouter } from "react-router-dom";

import { createAppStore } from "@/app/store";
import type { TaskOutcome } from "@/shared/types/app";

const useListTasksQueryMock = vi.fn();
const useGetTaskQueryMock = vi.fn();
const useAgentConsoleScopeMock = vi.fn();

vi.mock("@/shared/api/coreBridgeApi", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/coreBridgeApi")>();

  return {
    ...actual,
    useListTasksQuery: (...args: unknown[]) => useListTasksQueryMock(...args),
    useGetTaskQuery: (...args: unknown[]) => useGetTaskQueryMock(...args)
  };
});
vi.mock("@/shared/hooks", () => ({
  useAgentConsoleScope: () => useAgentConsoleScopeMock()
}));

import { TasksScreen } from "@/features/tasks/TasksScreen";

const baseTask: TaskOutcome = {
  task_id: "task-mock-1001",
  run_id: "run-mock-1001",
  agent_id: "default",
  state: "SUCCEEDED",
  node_id: "mock-local",
  error: "",
  replay: false,
  payload: {
    output: {
      text: "Generated overview summary from mock transport."
    }
  },
  started_at: "2026-03-10T10:20:00Z",
  finished_at: "2026-03-10T10:20:02Z"
};

function renderTasksScreen(preloadedState?: Parameters<typeof createAppStore>[0]) {
  const store = createAppStore(preloadedState);

  return render(
    <Provider store={store}>
      <MemoryRouter initialEntries={["/app/agents/default/tasks"]}>
        <TasksScreen />
      </MemoryRouter>
    </Provider>,
  );
}

describe("TasksScreen", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAgentConsoleScopeMock.mockReturnValue({
      activeAgentId: "default",
      defaultAgentId: "default",
      availableAgents: [{ agent_id: "default", is_default: true }],
      isLoading: false
    });
    useListTasksQueryMock.mockReturnValue({
      data: {
        tasks: [baseTask],
        count: 1
      },
      error: undefined,
      isLoading: false,
      isFetching: false,
      refetch: vi.fn()
    });
    useGetTaskQueryMock.mockImplementation((_args: { taskId: string }, options?: { skip?: boolean }) => ({
      data: options?.skip ? undefined : baseTask,
      error: undefined,
      isLoading: false,
      isFetching: false,
      refetch: vi.fn()
    }));
  });

  it("renders the recent task list and summary", () => {
    renderTasksScreen();

    expect(screen.getByText("Task Center")).toBeInTheDocument();
    expect(screen.getByText("task-mock-1001")).toBeInTheDocument();
    expect(screen.getByText("run-mock-1001")).toBeInTheDocument();
    expect(screen.getByText("Generated overview summary from mock transport.")).toBeInTheDocument();
    expect(useListTasksQueryMock).toHaveBeenCalledWith(
      { agentId: "default", limit: 20 },
      { skip: false },
    );
  });

  it("opens and closes the task detail drawer", async () => {
    const user = userEvent.setup();
    renderTasksScreen();

    await user.click(screen.getByRole("button", { name: /task-mock-1001/i }));

    expect(screen.getByText("Task Detail")).toBeInTheDocument();
    expect(screen.getAllByText("run-mock-1001").length).toBeGreaterThan(0);
    expect(screen.getByText((content) => content.includes('"output":'))).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Close" }));

    expect(screen.queryByText("Task Detail")).not.toBeInTheDocument();
  });

  it("enables polling for a running task and stops polling for a terminal task", async () => {
    const runningTask: TaskOutcome = {
      ...baseTask,
      state: "RUNNING",
      finished_at: null
    };

    useGetTaskQueryMock.mockImplementation((_args: { taskId: string }, options?: { skip?: boolean }) => ({
      data: options?.skip ? undefined : runningTask,
      error: undefined,
      isLoading: false,
      isFetching: false,
      refetch: vi.fn()
    }));

    const user = userEvent.setup();
    const { rerender } = renderTasksScreen();

    await user.click(screen.getByRole("button", { name: /task-mock-1001/i }));

    await waitFor(() => {
      const [, options] = useGetTaskQueryMock.mock.calls.at(-1) as [
        { taskId: string },
        { pollingInterval: number }
      ];
      expect(options.pollingInterval).toBe(5_000);
    });

    useGetTaskQueryMock.mockImplementation((_args: { taskId: string }, options?: { skip?: boolean }) => ({
      data: options?.skip ? undefined : baseTask,
      error: undefined,
      isLoading: false,
      isFetching: false,
      refetch: vi.fn()
    }));

    rerender(
      <Provider
        store={createAppStore({
          tasksUi: {
            activeTaskId: "task-mock-1001",
            listLimit: 20,
            autoRefresh: true
          }
        })}
      >
        <MemoryRouter initialEntries={["/app/agents/default/tasks?taskId=task-mock-1001"]}>
          <TasksScreen />
        </MemoryRouter>
      </Provider>,
    );

    await waitFor(() => {
      const [, options] = useGetTaskQueryMock.mock.calls.at(-1) as [
        { taskId: string },
        { pollingInterval: number }
      ];
      expect(options.pollingInterval).toBe(0);
    });
  });
});
