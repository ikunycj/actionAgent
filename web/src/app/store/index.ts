import { combineReducers, configureStore } from "@reduxjs/toolkit";
import { setupListeners } from "@reduxjs/toolkit/query";
import { useDispatch, useSelector, type TypedUseSelectorHook } from "react-redux";

import { listenerMiddleware } from "@/app/store/listenerMiddleware";
import { authReducer } from "@/app/store/slices/authSlice";
import { chatUiReducer } from "@/app/store/slices/chatUiSlice";
import { configDraftReducer } from "@/app/store/slices/configDraftSlice";
import { connectionReducer } from "@/app/store/slices/connectionSlice";
import { layoutReducer } from "@/app/store/slices/layoutSlice";
import { tasksUiReducer } from "@/app/store/slices/tasksUiSlice";
import { coreBridgeApi } from "@/shared/api/coreBridgeApi";
import { coreHttpApi } from "@/shared/api/coreHttpApi";

const reducer = {
  connection: connectionReducer,
  auth: authReducer,
  layout: layoutReducer,
  chatUi: chatUiReducer,
  configDraft: configDraftReducer,
  tasksUi: tasksUiReducer,
  [coreHttpApi.reducerPath]: coreHttpApi.reducer,
  [coreBridgeApi.reducerPath]: coreBridgeApi.reducer
};

const rootReducer = combineReducers(reducer);

type AppPreloadedState = Partial<ReturnType<typeof rootReducer>>;

export function createAppStore(preloadedState?: AppPreloadedState) {
  return configureStore({
    reducer: rootReducer,
    preloadedState: preloadedState as ReturnType<typeof rootReducer> | undefined,
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().concat(
        coreHttpApi.middleware,
        coreBridgeApi.middleware,
        listenerMiddleware.middleware,
      )
  });
}

export const store = createAppStore();

setupListeners(store.dispatch);

export type AppStore = typeof store;
export type RootState = ReturnType<AppStore["getState"]>;
export type AppDispatch = AppStore["dispatch"];

export const useAppDispatch = () => useDispatch<AppDispatch>();
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector;
