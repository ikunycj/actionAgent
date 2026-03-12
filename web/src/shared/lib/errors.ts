import type { AppError } from "@/shared/types/app";

export function getErrorMessage(error: AppError | { message?: string } | null | undefined) {
  return error?.message ?? "Unknown error";
}
