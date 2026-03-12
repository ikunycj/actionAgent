import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

import type { ConnectivityState } from "@/shared/types/app";
import { env } from "@/shared/config/env";
import { normalizeCoreBaseUrl } from "@/shared/lib/coreUrl";
import { localStore, storageKeys } from "@/shared/lib/storage";

function getInitialCoreBaseUrl() {
  return normalizeCoreBaseUrl(localStore.get(storageKeys.coreBaseUrl) ?? env.defaultCoreUrl);
}

type ConnectionState = {
  coreBaseUrl: string | null;
  lastConnectedAt: string | null;
  connectivityState: ConnectivityState;
};

const initialState: ConnectionState = {
  coreBaseUrl: getInitialCoreBaseUrl(),
  lastConnectedAt: null,
  connectivityState: "unknown"
};

const connectionSlice = createSlice({
  name: "connection",
  initialState,
  reducers: {
    setCoreBaseUrl(state, action: PayloadAction<string | null>) {
      state.coreBaseUrl = normalizeCoreBaseUrl(action.payload);
    },
    setLastConnectedAt(state, action: PayloadAction<string | null>) {
      state.lastConnectedAt = action.payload;
    },
    setConnectivityState(state, action: PayloadAction<ConnectivityState>) {
      state.connectivityState = action.payload;
    }
  }
});

export const { setConnectivityState, setCoreBaseUrl, setLastConnectedAt } =
  connectionSlice.actions;
export const connectionReducer = connectionSlice.reducer;
