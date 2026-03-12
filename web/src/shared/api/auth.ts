import type {
  AuthIdentity,
  AuthRole,
  AuthSession,
  LoginCredentials
} from "@/shared/types/app";

type UnknownRecord = Record<string, unknown>;

function asRecord(value: unknown): UnknownRecord | null {
  if (!value || typeof value !== "object") {
    return null;
  }

  return value as UnknownRecord;
}

function readString(record: UnknownRecord | null, keys: string[]) {
  for (const key of keys) {
    const value = record?.[key];

    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }

  return null;
}

function normalizeActor(value: unknown) {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }

  const record = asRecord(value);

  return readString(record, ["display_name", "displayName", "name", "username", "id", "sub"]);
}

function normalizeRole(value: unknown): AuthRole {
  if (value === "admin" || value === "viewer") {
    return value;
  }

  return "anonymous";
}

export function normalizeAuthIdentity(payload: unknown): AuthIdentity {
  const record = asRecord(payload);
  const actorSource = record?.actor ?? record?.user ?? record?.subject ?? record?.me ?? null;
  const actorRecord = asRecord(actorSource);

  return {
    role: normalizeRole(record?.role ?? actorRecord?.role),
    actor:
      normalizeActor(actorSource) ??
      readString(record, ["actor_name", "actorName", "username", "subject"])
  };
}

export function normalizeAuthSession(payload: unknown): AuthSession {
  const record = asRecord(payload);
  const identity = normalizeAuthIdentity(payload);

  return {
    accessToken: readString(record, ["access_token", "accessToken", "token"]),
    refreshToken: readString(record, ["refresh_token", "refreshToken"]),
    role: identity.role,
    actor: identity.actor
  };
}

export function mergeAuthSession(existing: AuthSession, incoming: AuthSession): AuthSession {
  return {
    accessToken: incoming.accessToken ?? existing.accessToken,
    refreshToken: incoming.refreshToken ?? existing.refreshToken,
    role: incoming.role === "anonymous" ? existing.role : incoming.role,
    actor: incoming.actor ?? existing.actor
  };
}

export function buildLoginRequest(credentials: LoginCredentials) {
  if (credentials.mode === "token") {
    return {
      mode: "token",
      token: credentials.token.trim()
    };
  }

  return {
    mode: "password",
    username: credentials.username.trim(),
    password: credentials.password
  };
}

export function buildRefreshRequest(refreshToken: string) {
  return {
    refresh_token: refreshToken
  };
}

export function buildLogoutRequest(refreshToken: string | null) {
  if (!refreshToken) {
    return {};
  }

  return {
    refresh_token: refreshToken
  };
}
