import { Navigate } from "react-router-dom";

import { useAgentConsoleScope } from "@/shared/hooks";

export function AgentEntryGate() {
  const scope = useAgentConsoleScope();

  if (scope.isLoading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <div className="max-w-md rounded-3xl border border-border/80 bg-panel p-6 text-center shadow-panel">
          <h1 className="text-lg font-semibold text-text">Resolving agent console</h1>
          <p className="mt-2 text-sm text-textMuted">
            WebUI is loading the active agent context from Core.
          </p>
        </div>
      </div>
    );
  }

  if (!scope.activeAgentId) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <div className="max-w-md rounded-3xl border border-border/80 bg-panel p-6 text-center shadow-panel">
          <h1 className="text-lg font-semibold text-text">No agent available</h1>
          <p className="mt-2 text-sm text-textMuted">
            Core did not return a usable default agent for the management console.
          </p>
        </div>
      </div>
    );
  }

  return <Navigate replace to={scope.buildScopedPath("/overview")} />;
}
