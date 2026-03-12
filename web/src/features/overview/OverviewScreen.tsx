import { useAgentConsoleScope } from "@/shared/hooks";
import { StatusBadge } from "@/shared/ui";

export function OverviewScreen() {
  const scope = useAgentConsoleScope();

  return (
    <section className="grid gap-4 md:grid-cols-3">
      <article className="space-y-3 rounded-3xl border border-border bg-panel p-6 shadow-panel">
        <div className="flex items-center justify-between">
          <h1 className="text-lg font-semibold">Agent overview</h1>
          <StatusBadge label={scope.activeAgentId ?? "Resolving"} tone="success" />
        </div>
        <p className="text-sm text-textMuted">
          This console is scoped to one active agent. Health, auth, and configuration readiness
          summaries will land here for the selected agent.
        </p>
      </article>
      <article className="space-y-3 rounded-3xl border border-border bg-panel p-6 shadow-panel">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Default routing</h2>
          <StatusBadge label={scope.defaultAgentId ?? "Pending"} tone="neutral" />
        </div>
        <p className="text-sm text-textMuted">
          Bundled WebUI resolves the active agent from the runtime catalog and falls back to the
          Core default agent when the requested target is missing.
        </p>
      </article>
      <article className="space-y-3 rounded-3xl border border-border bg-panel p-6 shadow-panel">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Console scope</h2>
          <StatusBadge label={`${scope.availableAgents.length} agents`} tone="neutral" />
        </div>
        <p className="text-sm text-textMuted">
          Configuration, history, diagnostics, and task views are expected to operate inside this
          agent boundary instead of acting as a global multi-agent dashboard.
        </p>
      </article>
    </section>
  );
}
