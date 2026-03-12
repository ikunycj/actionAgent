import { useAppSelector } from "@/app/store";
import { env } from "@/shared/config/env";
import { StatusBadge } from "@/shared/ui";

export function StatusStrip() {
  const connectivityState = useAppSelector((state) => state.connection.connectivityState);
  const role = useAppSelector((state) => state.auth.role);
  const sessionState = useAppSelector((state) => state.auth.sessionState);

  return (
    <div className="flex flex-wrap items-center gap-2">
      <StatusBadge
        label={`Core: ${connectivityState}`}
        tone={
          connectivityState === "online"
            ? "success"
            : connectivityState === "offline"
              ? "danger"
              : "neutral"
        }
      />
      <StatusBadge
        label={`Role: ${role}`}
        tone={role === "admin" ? "success" : role === "viewer" ? "warning" : "neutral"}
      />
      <StatusBadge
        label={`Session: ${sessionState}`}
        tone={sessionState === "checking" ? "warning" : "neutral"}
      />
      <StatusBadge
        label={`Data: ${env.apiMode}`}
        tone={env.apiMode === "mock" ? "warning" : env.apiMode === "hybrid" ? "neutral" : "success"}
      />
    </div>
  );
}
