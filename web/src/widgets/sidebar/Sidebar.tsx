import { NavLink } from "react-router-dom";

import { cn } from "@/shared/lib/cn";
import { useAgentConsoleScope } from "@/shared/hooks";

const navItems = [
  { path: "/overview", label: "Overview" },
  { path: "/diagnostics", label: "Diagnostics" },
  { path: "/history", label: "History" },
  { path: "/tasks", label: "Tasks" },
  { path: "/settings/model", label: "Settings" }
] as const;

export function Sidebar() {
  const scope = useAgentConsoleScope();

  if (!scope.activeAgentId) {
    return null;
  }

  return (
    <aside className="hidden w-64 shrink-0 rounded-[2rem] border border-border/80 bg-panel/90 p-4 shadow-panel backdrop-blur lg:block">
      <div className="mb-8 space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-textMuted">
          ActionAgent
        </p>
        <h1 className="text-xl font-semibold">Agent Console</h1>
        <p className="text-sm text-textMuted">Agent: {scope.activeAgentId}</p>
      </div>
      <nav className="space-y-2">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            className={({ isActive }) =>
              cn(
                "block rounded-2xl px-4 py-3 text-sm font-medium transition",
                isActive
                  ? "bg-accent text-white"
                  : "bg-transparent text-textMuted hover:bg-panelAlt hover:text-text",
              )
            }
            to={scope.buildScopedPath(item.path)}
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
