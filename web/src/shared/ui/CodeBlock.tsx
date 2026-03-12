import type { PropsWithChildren } from "react";

import { cn } from "@/shared/lib/cn";

type CodeBlockProps = PropsWithChildren<{
  className?: string;
}>;

export function CodeBlock({ children, className }: CodeBlockProps) {
  return (
    <pre
      className={cn(
        "overflow-x-auto rounded-2xl border border-border/80 bg-panelAlt/80 p-4 text-xs leading-6 text-text",
        className,
      )}
    >
      <code>{children}</code>
    </pre>
  );
}
