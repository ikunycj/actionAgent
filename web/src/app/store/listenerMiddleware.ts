import { createListenerMiddleware } from "@reduxjs/toolkit";

import { clearSession, setSession, updateIdentity } from "@/app/store/slices/authSlice";
import { setCoreBaseUrl } from "@/app/store/slices/connectionSlice";
import { coreBridgeApi } from "@/shared/api/coreBridgeApi";
import { coreHttpApi } from "@/shared/api/coreHttpApi";
import { localStore, sessionStore, storageKeys } from "@/shared/lib/storage";
import type { AuthSession } from "@/shared/types/app";

export const listenerMiddleware = createListenerMiddleware();

function persistSession(session: AuthSession) {
  if (session.accessToken) {
    sessionStore.set(storageKeys.accessToken, session.accessToken);
  } else {
    sessionStore.remove(storageKeys.accessToken);
  }

  if (session.refreshToken) {
    sessionStore.set(storageKeys.refreshToken, session.refreshToken);
  } else {
    sessionStore.remove(storageKeys.refreshToken);
  }

  if (session.role === "admin" || session.role === "viewer") {
    sessionStore.set(storageKeys.authRole, session.role);
  } else {
    sessionStore.remove(storageKeys.authRole);
  }

  if (session.actor) {
    sessionStore.set(storageKeys.actor, session.actor);
  } else {
    sessionStore.remove(storageKeys.actor);
  }
}

listenerMiddleware.startListening({
  actionCreator: setCoreBaseUrl,
  effect: async (action, api) => {
    if (action.payload) {
      localStore.set(storageKeys.coreBaseUrl, action.payload);
    } else {
      localStore.remove(storageKeys.coreBaseUrl);
    }

    api.dispatch(coreHttpApi.util.resetApiState());
    api.dispatch(coreBridgeApi.util.resetApiState());
  }
});

listenerMiddleware.startListening({
  matcher: (action) => setSession.match(action) || updateIdentity.match(action),
  effect: async (_, api) => {
    const state = api.getState() as { auth: AuthSession };
    persistSession(state.auth);
  }
});

listenerMiddleware.startListening({
  actionCreator: clearSession,
  effect: async (_, api) => {
    sessionStore.remove(storageKeys.accessToken);
    sessionStore.remove(storageKeys.refreshToken);
    sessionStore.remove(storageKeys.authRole);
    sessionStore.remove(storageKeys.actor);

    api.dispatch(coreHttpApi.util.resetApiState());
    api.dispatch(coreBridgeApi.util.resetApiState());
  }
});
