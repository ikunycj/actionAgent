import { useAgentConsoleScope } from "@/shared/hooks";

export function ConfigEditorScreen() {
  const scope = useAgentConsoleScope();

  return (
    <section className="rounded-3xl border border-border bg-panel p-6 shadow-panel">
      <h1 className="text-xl font-semibold">Model Settings</h1>
      <p className="mt-2 text-sm text-textMuted">
        Configuration editing will be implemented after auth and overview. This page is reserved
        for agent `{scope.activeAgentId ?? "resolving"}`.
      </p>
    </section>
  );
}
