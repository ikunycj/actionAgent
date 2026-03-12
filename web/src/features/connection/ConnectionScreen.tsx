import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useNavigate } from "react-router-dom";
import { z } from "zod";

import { useAppDispatch, useAppSelector } from "@/app/store";
import { clearSession } from "@/app/store/slices/authSlice";
import {
  setConnectivityState,
  setCoreBaseUrl,
  setLastConnectedAt
} from "@/app/store/slices/connectionSlice";
import { useCheckHealthMutation } from "@/shared/api/coreHttpApi";
import { normalizeCoreBaseUrl } from "@/shared/lib/coreUrl";
import { getErrorMessage } from "@/shared/lib/errors";
import type { AppError } from "@/shared/types/app";
import { Button, Input, StatusBadge } from "@/shared/ui";

const connectionFormSchema = z.object({
  baseUrl: z.string().trim().url("Enter a valid Core URL, for example http://127.0.0.1:8000")
});

type ConnectionFormValues = z.infer<typeof connectionFormSchema>;

export function ConnectionScreen() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const currentCoreBaseUrl = useAppSelector((state) => state.connection.coreBaseUrl);
  const lastConnectedAt = useAppSelector((state) => state.connection.lastConnectedAt);
  const connectivityState = useAppSelector((state) => state.connection.connectivityState);
  const accessToken = useAppSelector((state) => state.auth.accessToken);
  const [checkHealth, { isLoading }] = useCheckHealthMutation();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const form = useForm<ConnectionFormValues>({
    resolver: zodResolver(connectionFormSchema),
    defaultValues: {
      baseUrl: currentCoreBaseUrl ?? ""
    }
  });

  useEffect(() => {
    form.reset({
      baseUrl: currentCoreBaseUrl ?? ""
    });
  }, [currentCoreBaseUrl, form]);

  async function onSubmit(values: ConnectionFormValues) {
    const baseUrl = normalizeCoreBaseUrl(values.baseUrl);

    if (!baseUrl) {
      return;
    }

    dispatch(setConnectivityState("checking"));
    setSubmitError(null);

    try {
      const health = await checkHealth({ baseUrl }).unwrap();

      if (!health.ok || !health.ready) {
        dispatch(setConnectivityState("offline"));
        setSubmitError("Core is reachable but not ready yet.");
        return;
      }

      const changedCore = currentCoreBaseUrl !== baseUrl;

      dispatch(setCoreBaseUrl(baseUrl));
      dispatch(setLastConnectedAt(health.ts ?? new Date().toISOString()));
      dispatch(setConnectivityState("online"));

      if (changedCore) {
        dispatch(clearSession());
      }

      navigate(changedCore || !accessToken ? "/login" : "/app", {
        replace: true
      });
    } catch (error) {
      dispatch(setConnectivityState("offline"));
      setSubmitError(getErrorMessage(error as AppError));
    }
  }

  return (
    <section className="w-full space-y-6 rounded-3xl border border-border bg-panel p-6 shadow-panel">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-textMuted">
            Advanced
          </p>
          <h1 className="text-xl font-semibold">Link To Another Core</h1>
        </div>
        <StatusBadge
          label={`Connection: ${connectivityState}`}
          tone={
            connectivityState === "online"
              ? "success"
              : connectivityState === "offline"
                ? "danger"
                : "neutral"
          }
        />
      </div>
      <div className="space-y-1">
        <p className="text-sm text-textMuted">
          Bundled WebUI normally uses the same origin as Core. Use this page only for standalone
          development or when you need to point the browser at a different Core instance.
        </p>
      </div>
      {currentCoreBaseUrl ? (
        <div className="rounded-2xl border border-border/80 bg-panelAlt/60 p-4 text-sm text-textMuted">
          <p className="font-medium text-text">Saved Core</p>
          <p className="mt-1 break-all">{currentCoreBaseUrl}</p>
          {lastConnectedAt ? <p className="mt-2">Last successful probe: {lastConnectedAt}</p> : null}
        </div>
      ) : null}
      <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
        <div className="space-y-2">
          <label className="block text-sm font-medium text-text" htmlFor="core-url">
            Core Base URL
          </label>
          <Input
            id="core-url"
            placeholder="http://127.0.0.1:8000"
            {...form.register("baseUrl")}
          />
          {form.formState.errors.baseUrl ? (
            <p className="text-sm text-danger">{form.formState.errors.baseUrl.message}</p>
          ) : null}
        </div>
        {submitError ? <p className="text-sm text-danger">{submitError}</p> : null}
        <div className="flex flex-wrap gap-3">
          <Button disabled={isLoading} type="submit">
            {isLoading ? "Checking..." : "Check connectivity"}
          </Button>
          {currentCoreBaseUrl ? (
            <Button asChild tone="secondary">
              <Link to={accessToken ? "/app" : "/login"}>
                Continue with saved Core
              </Link>
            </Button>
          ) : null}
        </div>
      </form>
      <div className="rounded-2xl border border-dashed border-border p-4 text-sm text-textMuted">
        <p className="font-medium text-text">Expected endpoint</p>
        <p className="mt-1">
          The WebUI probes `GET /healthz` before it saves an alternate Core address.
        </p>
      </div>
    </section>
  );
}
