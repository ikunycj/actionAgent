export const storageKeys = {
  coreBaseUrl: "actionagent.webui.core-base-url",
  accessToken: "actionagent.webui.access-token",
  refreshToken: "actionagent.webui.refresh-token",
  authRole: "actionagent.webui.auth-role",
  actor: "actionagent.webui.actor"
} as const;

function getStorage(type: "session" | "local") {
  if (typeof window === "undefined") {
    return null;
  }

  return type === "session" ? window.sessionStorage : window.localStorage;
}

function readValue(type: "session" | "local", key: string) {
  try {
    return getStorage(type)?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function writeValue(type: "session" | "local", key: string, value: string) {
  try {
    getStorage(type)?.setItem(key, value);
  } catch {
    // Ignore storage write failures and keep runtime state in memory.
  }
}

function removeValue(type: "session" | "local", key: string) {
  try {
    getStorage(type)?.removeItem(key);
  } catch {
    // Ignore storage cleanup failures and keep runtime state in memory.
  }
}

export const sessionStore = {
  get(key: string) {
    return readValue("session", key);
  },
  set(key: string, value: string) {
    writeValue("session", key, value);
  },
  remove(key: string) {
    removeValue("session", key);
  }
};

export const localStore = {
  get(key: string) {
    return readValue("local", key);
  },
  set(key: string, value: string) {
    writeValue("local", key, value);
  },
  remove(key: string) {
    removeValue("local", key);
  }
};
