import type { ApiMode } from "@/shared/types/app";

function parseNumber(value: string | undefined, fallback: number) {
  const parsed = Number(value ?? String(fallback));
  return Number.isFinite(parsed) ? parsed : fallback;
}

function parseApiMode(value: string | undefined): ApiMode {
  if (value === "mock" || value === "hybrid" || value === "live") {
    return value;
  }

  return "live";
}

function resolveDefaultCoreUrl() {
  const explicit = import.meta.env.VITE_DEFAULT_CORE_URL?.trim();
  if (explicit) {
    return explicit;
  }
  if (import.meta.env.PROD && typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  return null;
}

const defaultCoreUrl = resolveDefaultCoreUrl();
const requestTimeoutMs = parseNumber(import.meta.env.VITE_REQUEST_TIMEOUT_MS, 15000);
const mockDelayMs = parseNumber(import.meta.env.VITE_MOCK_DELAY_MS, 120);

export const env = {
  appTitle: import.meta.env.VITE_APP_TITLE?.trim() || "ActionAgent WebUI",
  defaultCoreUrl,
  bundledOriginMode: Boolean(defaultCoreUrl && !import.meta.env.VITE_DEFAULT_CORE_URL?.trim()),
  requestTimeoutMs,
  apiMode: parseApiMode(import.meta.env.VITE_API_MODE),
  mockDelayMs
} as const;
