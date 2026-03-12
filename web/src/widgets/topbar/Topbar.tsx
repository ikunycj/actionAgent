import { Link } from "react-router-dom";

import { useAppSelector } from "@/app/store";
import { useLogoutMutation } from "@/shared/api/coreHttpApi";
import { useAgentConsoleScope } from "@/shared/hooks";
import { Button } from "@/shared/ui";
import { StatusStrip } from "@/widgets/status-strip";

export function Topbar() {
  const coreBaseUrl = useAppSelector((state) => state.connection.coreBaseUrl);
  const refreshToken = useAppSelector((state) => state.auth.refreshToken);
  const actor = useAppSelector((state) => state.auth.actor);
  const role = useAppSelector((state) => state.auth.role);
  const [logout, { isLoading }] = useLogoutMutation();
  const scope = useAgentConsoleScope();

  async function handleLogout() {
    await logout({ refreshToken }).unwrap().catch(() => undefined);
  }

  return (
    <header className="flex flex-col gap-3 rounded-[2rem] border border-border/80 bg-panel/90 px-5 py-4 shadow-panel backdrop-blur md:flex-row md:items-center md:justify-between">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.2em] text-textMuted">
          Control plane
        </p>
        <div className="mt-1 flex flex-wrap gap-3 text-sm text-textMuted">
          <span>Core: {coreBaseUrl ?? "Not connected"}</span>
          <span>Agent: {scope.activeAgentId ?? "Resolving"}</span>
          <span>Actor: {actor ?? "Local operator"}</span>
          <span>Role: {role}</span>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <StatusStrip />
        {scope.availableAgents.length > 1
          ? scope.availableAgents.map((agent) => (
              <Button
                asChild
                key={agent.agent_id}
                tone={agent.agent_id === scope.activeAgentId ? "primary" : "ghost"}
              >
                <Link to={`/app/agents/${agent.agent_id}/overview`}>{agent.agent_id}</Link>
              </Button>
            ))
          : null}
        <Button asChild tone="ghost">
          <Link to="/connect">Advanced Core Link</Link>
        </Button>
        {refreshToken ? (
          <Button disabled={isLoading} onClick={handleLogout} tone="secondary">
            {isLoading ? "Signing out..." : "Logout"}
          </Button>
        ) : null}
      </div>
    </header>
  );
}
