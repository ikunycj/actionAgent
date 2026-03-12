import { Navigate, createBrowserRouter } from "react-router-dom";

import { RequireConnection } from "@/app/router/guards";
import { AgentEntryGate } from "@/features/agent-scope/AgentEntryGate";
import { ChatPage } from "@/pages/chat";
import { ConnectPage } from "@/pages/connect";
import { HistoryPage } from "@/pages/history";
import { LoginPage } from "@/pages/login";
import { OverviewPage } from "@/pages/overview";
import { SettingsPage } from "@/pages/settings";
import { TasksPage } from "@/pages/tasks";
import { AppShell } from "@/widgets/app-shell";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Navigate replace to="/app" />
  },
  {
    path: "/connect",
    element: <ConnectPage />
  },
  {
    path: "/login",
    element: <LoginPage />
  },
  {
    path: "/app",
    element: (
      <RequireConnection>
        <AgentEntryGate />
      </RequireConnection>
    ),
  },
  {
    path: "/app/agents/:agentId",
    element: (
      <RequireConnection>
        <AppShell />
      </RequireConnection>
    ),
    children: [
      {
        index: true,
        element: <Navigate replace to="overview" />
      },
      {
        path: "overview",
        element: <OverviewPage />
      },
      {
        path: "diagnostics",
        element: <ChatPage />
      },
      {
        path: "diagnostics/:sessionKey",
        element: <ChatPage />
      },
      {
        path: "history",
        element: <HistoryPage />
      },
      {
        path: "tasks",
        element: <TasksPage />
      },
      {
        path: "settings/model",
        element: <SettingsPage />
      }
    ]
  }
]);
