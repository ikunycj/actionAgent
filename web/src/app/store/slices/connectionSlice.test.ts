import { storageKeys } from "@/shared/lib/storage";

describe("connectionSlice", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.resetModules();
  });

  it("hydrates the saved Core URL from localStorage", async () => {
    window.localStorage.setItem(storageKeys.coreBaseUrl, "http://127.0.0.1:8000/");

    const { connectionReducer } = await import("./connectionSlice");
    const state = connectionReducer(undefined, { type: "init" });

    expect(state.coreBaseUrl).toBe("http://127.0.0.1:8000");
  });
});
