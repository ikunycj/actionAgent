import type { PropsWithChildren } from "react";
import { Navigate, useLocation } from "react-router-dom";

import { useAppSelector } from "@/app/store";

function GuardScreen({
  title,
  description
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <div className="max-w-md rounded-3xl border border-border/80 bg-panel p-6 text-center shadow-panel">
        <h1 className="text-lg font-semibold text-text">{title}</h1>
        <p className="mt-2 text-sm text-textMuted">{description}</p>
      </div>
    </div>
  );
}

export function RequireConnection({ children }: PropsWithChildren) {
  const location = useLocation();
  const coreBaseUrl = useAppSelector((state) => state.connection.coreBaseUrl);

  if (!coreBaseUrl) {
    return <Navigate replace state={{ from: location.pathname }} to="/connect" />;
  }

  return children;
}

export function RequireAuth({ children }: PropsWithChildren) {
  const location = useLocation();
  const accessToken = useAppSelector((state) => state.auth.accessToken);
  const sessionState = useAppSelector((state) => state.auth.sessionState);

  if (accessToken && sessionState === "checking") {
    return (
      <GuardScreen
        description="WebUI is validating the current browser session with Core."
        title="Restoring session"
      />
    );
  }

  if (!accessToken) {
    return <Navigate replace state={{ from: location.pathname }} to="/login" />;
  }

  return children;
}

export function RequireAdmin({ children }: PropsWithChildren) {
  const role = useAppSelector((state) => state.auth.role);

  if (role !== "admin") {
    return <Navigate replace to="/app" />;
  }

  return children;
}
