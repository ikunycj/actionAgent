import { render, screen } from "@testing-library/react";
import { Provider } from "react-redux";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { createAppStore } from "@/app/store";
import { RequireAdmin, RequireAuth, RequireConnection } from "@/app/router/guards";

function renderGuardRoute({
  initialEntry,
  preloadedState
}: {
  initialEntry: string;
  preloadedState?: Parameters<typeof createAppStore>[0];
}) {
  const store = createAppStore(preloadedState);

  return render(
    <Provider store={store}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route element={<div>Connect page</div>} path="/connect" />
          <Route element={<div>Login page</div>} path="/login" />
          <Route element={<div>App page</div>} path="/app" />
          <Route
            element={
              <RequireConnection>
                <RequireAuth>
                  <div>Protected overview</div>
                </RequireAuth>
              </RequireConnection>
            }
            path="/app/protected"
          />
          <Route
            element={
              <RequireConnection>
                <RequireAuth>
                  <RequireAdmin>
                    <div>Settings page</div>
                  </RequireAdmin>
                </RequireAuth>
              </RequireConnection>
            }
            path="/app/settings/model"
          />
        </Routes>
      </MemoryRouter>
    </Provider>,
  );
}

describe("route guards", () => {
  it("redirects to /connect when no Core URL is configured", () => {
    renderGuardRoute({
      initialEntry: "/app/protected",
      preloadedState: {
        connection: {
          coreBaseUrl: null,
          lastConnectedAt: null,
          connectivityState: "unknown"
        }
      }
    });

    expect(screen.getByText("Connect page")).toBeInTheDocument();
  });

  it("redirects to /login when there is no access token", () => {
    renderGuardRoute({
      initialEntry: "/app/protected",
      preloadedState: {
        connection: {
          coreBaseUrl: "http://core.test",
          lastConnectedAt: null,
          connectivityState: "online"
        },
        auth: {
          accessToken: null,
          refreshToken: null,
          role: "anonymous",
          actor: null,
          sessionState: "anonymous"
        }
      }
    });

    expect(screen.getByText("Login page")).toBeInTheDocument();
  });

  it("shows a restore screen while the session is being checked", () => {
    renderGuardRoute({
      initialEntry: "/app/protected",
      preloadedState: {
        connection: {
          coreBaseUrl: "http://core.test",
          lastConnectedAt: null,
          connectivityState: "online"
        },
        auth: {
          accessToken: "token-1",
          refreshToken: "refresh-1",
          role: "viewer",
          actor: "alice",
          sessionState: "checking"
        }
      }
    });

    expect(screen.getByText("Restoring session")).toBeInTheDocument();
  });

  it("blocks viewer access to admin-only settings", () => {
    renderGuardRoute({
      initialEntry: "/app/settings/model",
      preloadedState: {
        connection: {
          coreBaseUrl: "http://core.test",
          lastConnectedAt: null,
          connectivityState: "online"
        },
        auth: {
          accessToken: "token-1",
          refreshToken: "refresh-1",
          role: "viewer",
          actor: "alice",
          sessionState: "authenticated"
        }
      }
    });

    expect(screen.getByText("App page")).toBeInTheDocument();
  });
});
