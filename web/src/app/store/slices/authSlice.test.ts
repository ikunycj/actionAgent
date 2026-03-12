import { storageKeys } from "@/shared/lib/storage";

describe("authSlice", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    vi.resetModules();
  });

  it("hydrates the auth session from sessionStorage", async () => {
    window.sessionStorage.setItem(storageKeys.accessToken, "access-1");
    window.sessionStorage.setItem(storageKeys.refreshToken, "refresh-1");
    window.sessionStorage.setItem(storageKeys.authRole, "admin");
    window.sessionStorage.setItem(storageKeys.actor, "alice");

    const { authReducer } = await import("./authSlice");
    const state = authReducer(undefined, { type: "init" });

    expect(state.accessToken).toBe("access-1");
    expect(state.refreshToken).toBe("refresh-1");
    expect(state.role).toBe("admin");
    expect(state.actor).toBe("alice");
    expect(state.sessionState).toBe("checking");
  });
});
