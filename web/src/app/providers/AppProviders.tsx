import type { PropsWithChildren } from "react";
import { Provider } from "react-redux";

import "@/app/styles/index.css";

import { AppBootstrap } from "@/app/providers/AppBootstrap";
import { store } from "@/app/store";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <Provider store={store}>
      <AppBootstrap />
      {children}
    </Provider>
  );
}
