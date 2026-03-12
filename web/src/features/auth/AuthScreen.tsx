import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { z } from "zod";

import { useAppSelector } from "@/app/store";
import { useLoginMutation } from "@/shared/api/coreHttpApi";
import { getErrorMessage } from "@/shared/lib/errors";
import type { AppError, LoginCredentials } from "@/shared/types/app";
import { Button, Input, StatusBadge } from "@/shared/ui";

const loginFormSchema = z.discriminatedUnion("mode", [
  z.object({
    mode: z.literal("token"),
    token: z.string().trim().min(1, "Token is required"),
    username: z.string().optional(),
    password: z.string().optional()
  }),
  z.object({
    mode: z.literal("password"),
    token: z.string().optional(),
    username: z.string().trim().min(1, "Username is required"),
    password: z.string().min(1, "Password is required")
  })
]);

type LoginFormValues = z.infer<typeof loginFormSchema>;

function getRedirectTarget(value: unknown) {
  if (value && typeof value === "object" && "from" in value && typeof value.from === "string") {
    return value.from;
  }

  return "/app";
}

export function AuthScreen() {
  const navigate = useNavigate();
  const location = useLocation();
  const coreBaseUrl = useAppSelector((state) => state.connection.coreBaseUrl);
  const accessToken = useAppSelector((state) => state.auth.accessToken);
  const sessionState = useAppSelector((state) => state.auth.sessionState);
  const [login, { isLoading }] = useLoginMutation();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const redirectTarget = getRedirectTarget(location.state);
  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginFormSchema),
    defaultValues: {
      mode: "token",
      token: "",
      username: "",
      password: ""
    } as LoginFormValues
  });
  const mode = form.watch("mode");

  useEffect(() => {
    if (!coreBaseUrl) {
      navigate("/connect", { replace: true });
    }
  }, [coreBaseUrl, navigate]);

  useEffect(() => {
    if (accessToken && sessionState !== "checking") {
      navigate(redirectTarget, { replace: true });
    }
  }, [accessToken, navigate, redirectTarget, sessionState]);

  async function onSubmit(values: LoginFormValues) {
    setSubmitError(null);

    const credentials: LoginCredentials =
      values.mode === "token"
        ? {
            mode: "token",
            token: values.token
          }
        : {
            mode: "password",
            username: values.username,
            password: values.password
          };

    try {
      await login(credentials).unwrap();
      navigate(redirectTarget, { replace: true });
    } catch (error) {
      setSubmitError(getErrorMessage(error as AppError));
    }
  }

  function switchMode(nextMode: "token" | "password") {
    setSubmitError(null);
    form.clearErrors();
    form.setValue("mode", nextMode, {
      shouldDirty: true,
      shouldValidate: true
    });
  }

  return (
    <section className="w-full space-y-6 rounded-3xl border border-border bg-panel p-6 shadow-panel">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-textMuted">
            Step 2
          </p>
          <h1 className="text-xl font-semibold">Login</h1>
        </div>
        <StatusBadge
          label={sessionState === "checking" ? "Session: restoring" : "Session: ready"}
          tone={sessionState === "checking" ? "warning" : "neutral"}
        />
      </div>
      <div className="rounded-2xl border border-border/80 bg-panelAlt/60 p-4 text-sm text-textMuted">
        <p className="font-medium text-text">Current Core</p>
        <p className="mt-1 break-all">{coreBaseUrl ?? "Not configured"}</p>
        <div className="mt-3">
          <Button asChild tone="ghost">
            <Link to="/connect">Change Core</Link>
          </Button>
        </div>
      </div>
      <div className="flex gap-2 rounded-2xl border border-border/80 bg-panelAlt/50 p-2">
        <Button
          className="flex-1"
          disabled={isLoading}
          onClick={() => switchMode("token")}
          tone={mode === "token" ? "primary" : "secondary"}
        >
          Token
        </Button>
        <Button
          className="flex-1"
          disabled={isLoading}
          onClick={() => switchMode("password")}
          tone={mode === "password" ? "primary" : "secondary"}
        >
          Password
        </Button>
      </div>
      <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
        <input type="hidden" {...form.register("mode")} />
        {mode === "token" ? (
          <div className="space-y-2">
            <label className="block text-sm font-medium text-text" htmlFor="login-token">
              Access token
            </label>
            <Input
              id="login-token"
              placeholder="Paste an access token"
              type="password"
              {...form.register("token")}
            />
            {form.formState.errors.token ? (
              <p className="text-sm text-danger">{form.formState.errors.token.message}</p>
            ) : null}
          </div>
        ) : (
          <>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-text" htmlFor="login-username">
                Username
              </label>
              <Input
                id="login-username"
                placeholder="admin"
                {...form.register("username")}
              />
              {form.formState.errors.username ? (
                <p className="text-sm text-danger">{form.formState.errors.username.message}</p>
              ) : null}
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-text" htmlFor="login-password">
                Password
              </label>
              <Input
                id="login-password"
                placeholder="Password"
                type="password"
                {...form.register("password")}
              />
              {form.formState.errors.password ? (
                <p className="text-sm text-danger">{form.formState.errors.password.message}</p>
              ) : null}
            </div>
          </>
        )}
        {submitError ? <p className="text-sm text-danger">{submitError}</p> : null}
        <Button disabled={isLoading || !coreBaseUrl} type="submit">
          {isLoading ? "Signing in..." : "Sign in"}
        </Button>
      </form>
    </section>
  );
}
