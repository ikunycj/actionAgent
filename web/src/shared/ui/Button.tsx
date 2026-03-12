import { Slot } from "@radix-ui/react-slot";
import type { ButtonHTMLAttributes, PropsWithChildren } from "react";

import { cn } from "@/shared/lib/cn";

type ButtonProps = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    asChild?: boolean;
    tone?: "primary" | "secondary" | "ghost";
  }
>;

export function Button({
  asChild = false,
  className,
  tone = "primary",
  type = "button",
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot : "button";

  return (
    <Comp
      className={cn(
        "inline-flex items-center justify-center rounded-xl border px-4 py-2 text-sm font-semibold transition",
        "disabled:cursor-not-allowed disabled:opacity-60",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2",
        tone === "primary" &&
          "border-accent bg-accent text-white shadow-panel hover:bg-accent/90",
        tone === "secondary" &&
          "border-border bg-panel text-text hover:bg-panelAlt",
        tone === "ghost" &&
          "border-transparent bg-transparent text-text hover:bg-panelAlt",
        className,
      )}
      type={type}
      {...props}
    />
  );
}
