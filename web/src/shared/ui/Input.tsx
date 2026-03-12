import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "@/shared/lib/cn";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => {
    return (
      <input
        ref={ref}
        className={cn(
          "w-full rounded-xl border-border bg-panel px-3 py-2 text-sm text-text shadow-sm",
          "disabled:cursor-not-allowed disabled:opacity-60",
          "placeholder:text-textMuted focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/20",
          className,
        )}
        {...props}
      />
    );
  },
);

Input.displayName = "Input";
