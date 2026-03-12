import { cn } from "@/shared/lib/cn";
import type { StatusTone } from "@/shared/types/app";

type StatusBadgeProps = {
  label: string;
  tone?: StatusTone;
};

export function StatusBadge({ label, tone = "neutral" }: StatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium",
        tone === "neutral" && "border-border bg-panelAlt text-textMuted",
        tone === "success" && "border-success/20 bg-success/10 text-success",
        tone === "warning" && "border-warning/20 bg-warning/10 text-warning",
        tone === "danger" && "border-danger/20 bg-danger/10 text-danger",
      )}
    >
      {label}
    </span>
  );
}
