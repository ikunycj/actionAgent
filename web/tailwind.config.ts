import forms from "@tailwindcss/forms";
import typography from "@tailwindcss/typography";
import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: "hsl(var(--color-bg) / <alpha-value>)",
        panel: "hsl(var(--color-panel) / <alpha-value>)",
        panelAlt: "hsl(var(--color-panel-alt) / <alpha-value>)",
        border: "hsl(var(--color-border) / <alpha-value>)",
        text: "hsl(var(--color-text) / <alpha-value>)",
        textMuted: "hsl(var(--color-text-muted) / <alpha-value>)",
        accent: "hsl(var(--color-accent) / <alpha-value>)",
        accentSoft: "hsl(var(--color-accent-soft) / <alpha-value>)",
        success: "hsl(var(--color-success) / <alpha-value>)",
        warning: "hsl(var(--color-warning) / <alpha-value>)",
        danger: "hsl(var(--color-danger) / <alpha-value>)"
      },
      borderRadius: {
        xl: "1rem"
      },
      boxShadow: {
        panel: "0 20px 45px -28px rgba(15, 23, 42, 0.45)"
      },
      fontFamily: {
        sans: ["'Segoe UI'", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["'Cascadia Code'", "ui-monospace", "SFMono-Regular", "monospace"]
      }
    }
  },
  plugins: [forms, typography]
};

export default config;
