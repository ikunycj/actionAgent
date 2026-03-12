import { Outlet } from "react-router-dom";

import { Sidebar } from "@/widgets/sidebar";
import { Topbar } from "@/widgets/topbar";

export function AppShell() {
  return (
    <div className="min-h-screen bg-transparent">
      <div className="mx-auto flex min-h-screen max-w-[1600px] gap-4 p-4 lg:p-6">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <Topbar />
          <main className="min-h-[calc(100vh-8rem)] rounded-[2rem] border border-border/80 bg-panel/80 p-5 shadow-panel backdrop-blur">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}
