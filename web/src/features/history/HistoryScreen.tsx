import { useAgentConsoleScope } from "@/shared/hooks";

export function HistoryScreen() {
  const scope = useAgentConsoleScope();

  return (
    <section className="rounded-3xl border border-border bg-panel p-6 shadow-panel">
      <h1 className="text-xl font-semibold">History</h1>
      <p className="mt-2 text-sm text-textMuted">
        Session list and transcript panels will arrive in the history phase. They will be filtered
        to the active agent scope: {scope.activeAgentId ?? "resolving"}.
      </p>
    </section>
  );
}
