import { TOKEN_STORAGE_KEY } from "./token";

interface MutableGlobal {
  fetch?: jest.Mock;
  localStorage?: Storage;
}

function fakeStorage(values: Record<string, string>): Storage {
  return {
    getItem: (key: string) => values[key] ?? null,
    setItem: () => {},
    removeItem: () => {},
    clear: () => {},
    key: () => null,
    length: 0,
  } as Storage;
}

describe("apiFetch", () => {
  const originalEnv = process.env;
  const mutable = global as unknown as MutableGlobal;

  beforeEach(() => {
    jest.resetModules();
    process.env = {
      ...originalEnv,
      NEXT_PUBLIC_API_URL: "https://api.example.com",
      NEXT_PUBLIC_GOOGLE_CLIENT_ID: "client-id",
    };
  });

  afterEach(() => {
    process.env = originalEnv;
    delete mutable.fetch;
    delete mutable.localStorage;
  });

  it("attaches the stored bearer token", async () => {
    // Arrange
    const fetchMock = jest.fn().mockResolvedValue({ ok: true });
    mutable.fetch = fetchMock;
    mutable.localStorage = fakeStorage({ [TOKEN_STORAGE_KEY]: "tok123" });
    const { apiFetch } = require("./api");

    // Act
    await apiFetch("/api/files");

    // Assert
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("https://api.example.com/api/files");
    expect((init.headers as Headers).get("Authorization")).toBe("Bearer tok123");
  });

  it("omits the header when no token is stored", async () => {
    // Arrange
    const fetchMock = jest.fn().mockResolvedValue({ ok: true });
    mutable.fetch = fetchMock;
    mutable.localStorage = fakeStorage({});
    const { apiFetch } = require("./api");

    // Act
    await apiFetch("/api/files");

    // Assert
    const [, init] = fetchMock.mock.calls[0];
    expect((init.headers as Headers).has("Authorization")).toBe(false);
  });
});
