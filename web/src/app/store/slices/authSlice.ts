import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

import type { AuthIdentity, AuthSession, AuthSessionState } from "@/shared/types/app";
import { sessionStore, storageKeys } from "@/shared/lib/storage";

function getStoredAccessToken() {
  return sessionStore.get(storageKeys.accessToken);
}

function getStoredRefreshToken() {
  return sessionStore.get(storageKeys.refreshToken);
}

function getStoredRole(accessToken: string | null) {
  const role = sessionStore.get(storageKeys.authRole);

  if (!accessToken) {
    return "anonymous";
  }

  return role === "admin" || role === "viewer" ? role : "anonymous";
}

function getStoredActor(accessToken: string | null) {
  if (!accessToken) {
    return null;
  }

  return sessionStore.get(storageKeys.actor);
}

type AuthState = {
  accessToken: string | null;
  refreshToken: string | null;
  role: AuthIdentity["role"];
  actor: string | null;
  sessionState: AuthSessionState;
};

const storedAccessToken = getStoredAccessToken();

const initialState: AuthState = {
  accessToken: storedAccessToken,
  refreshToken: getStoredRefreshToken(),
  role: getStoredRole(storedAccessToken),
  actor: getStoredActor(storedAccessToken),
  sessionState: storedAccessToken ? "checking" : "anonymous"
};

const authSlice = createSlice({
  name: "auth",
  initialState,
  reducers: {
    setSession(state, action: PayloadAction<AuthSession>) {
      state.accessToken = action.payload.accessToken;
      state.refreshToken = action.payload.refreshToken;
      state.role = action.payload.role;
      state.actor = action.payload.actor;
      state.sessionState = action.payload.accessToken ? "authenticated" : "anonymous";
    },
    updateIdentity(state, action: PayloadAction<AuthIdentity>) {
      state.role = action.payload.role;
      state.actor = action.payload.actor;
      state.sessionState = state.accessToken ? "authenticated" : "anonymous";
    },
    setSessionState(state, action: PayloadAction<AuthSessionState>) {
      state.sessionState = action.payload;
    },
    finishSessionCheck(state) {
      state.sessionState = state.accessToken ? "authenticated" : "anonymous";
    },
    clearSession(state) {
      state.accessToken = null;
      state.refreshToken = null;
      state.role = "anonymous";
      state.actor = null;
      state.sessionState = "anonymous";
    }
  }
});

export const { clearSession, finishSessionCheck, setSession, setSessionState, updateIdentity } =
  authSlice.actions;
export const authReducer = authSlice.reducer;
