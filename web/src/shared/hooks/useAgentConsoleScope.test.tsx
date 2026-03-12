import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

const useGetRuntimeAgentsQueryMock = vi.fn();

vi.mock("@/shared/api/coreHttpApi", () => ({
  useGetRuntimeAgentsQuery: () => useGetRuntimeAgentsQueryMock()
}));

import { useAgentConsoleScope } from "@/shared/hooks";

function Probe() {
  const scope = useAgentConsoleScope();

  return (
    <div>
      <span data-testid="active-agent">{scope.activeAgentId ?? "none"}</span>
      <span data-testid="default-agent">{scope.defaultAgentId ?? "none"}</span>
      <span data-testid="is-fallback">{String(scope.isFallback)}</span>
      <span data-testid="scoped-path">{scope.buildScopedPath("/tasks")}</span>
    </div>
  );
}

function renderProbe(initialEntry: string, routePath: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route element={<Probe />} path={routePath} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("useAgentConsoleScope", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useGetRuntimeAgentsQueryMock.mockReturnValue({
      data: {
        default_agent: "default",
        agents: [
          { agent_id: "default", is_default: true },
          { agent_id: "alpha", is_default: false }
        ],
        count: 2
      },
      isLoading: false,
      isError: false,
      error: undefined
    });
  });

  it("falls back to the default agent when no explicit target is provided", () => {
    renderProbe("/app", "/app");

    expect(screen.getByTestId("active-agent")).toHaveTextContent("default");
    expect(screen.getByTestId("default-agent")).toHaveTextContent("default");
    expect(screen.getByTestId("is-fallback")).toHaveTextContent("false");
    expect(screen.getByTestId("scoped-path")).toHaveTextContent("/app/agents/default/tasks");
  });

  it("uses the requested agent from query parameters when it exists", () => {
    renderProbe("/app?agent=alpha", "/app");

    expect(screen.getByTestId("active-agent")).toHaveTextContent("alpha");
    expect(screen.getByTestId("is-fallback")).toHaveTextContent("false");
    expect(screen.getByTestId("scoped-path")).toHaveTextContent("/app/agents/alpha/tasks");
  });

  it("falls back to the default agent when the route agent is unknown", () => {
    renderProbe("/app/agents/missing/overview", "/app/agents/:agentId/overview");

    expect(screen.getByTestId("active-agent")).toHaveTextContent("default");
    expect(screen.getByTestId("is-fallback")).toHaveTextContent("true");
    expect(screen.getByTestId("scoped-path")).toHaveTextContent("/app/agents/default/tasks");
  });
});
