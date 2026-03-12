import { useAgentConsoleScope } from "@/shared/hooks";

export function ChatWorkspace() {
  const scope = useAgentConsoleScope();

  return (
    <section className="rounded-3xl border border-border bg-panel p-6 shadow-panel">
      <h1 className="text-xl font-semibold">Diagnostics</h1>
      <p className="mt-2 text-sm text-textMuted">
        This surface is no longer the main user chat entry. It will become an agent-scoped
        diagnostic probe for `{scope.activeAgentId ?? "resolving"}` and model troubleshooting.
      </p>
    </section>
  );
}
