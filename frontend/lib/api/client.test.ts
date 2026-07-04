import { afterEach, describe, expect, it, vi } from "vitest";

import { createApiClient } from "./client";

const config = {
  apiUrl: "https://api.example.com",
  cognitoUserPoolId: "pool",
  cognitoClientId: "client",
};

function jsonResponse(status: number, body: unknown = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("createApiClient", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("calls onUnauthorized when the API returns 401", async () => {
    // Arrange
    const onUnauthorized = vi.fn();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(401, { error: "invalid token" })));
    const api = createApiClient(config, () => "token", onUnauthorized);

    // Act
    await api.GET("/files", {});

    // Assert
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });

  it("does not call onUnauthorized on a successful response", async () => {
    // Arrange
    const onUnauthorized = vi.fn();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(200, { files: [] })));
    const api = createApiClient(config, () => "token", onUnauthorized);

    // Act
    await api.GET("/files", {});

    // Assert
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it("attaches the bearer token to the request", async () => {
    // Arrange
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { files: [] }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createApiClient(config, () => "my-token", vi.fn());

    // Act
    await api.GET("/files", {});

    // Assert
    const request = fetchMock.mock.calls[0][0] as Request;
    expect(request.headers.get("Authorization")).toBe("Bearer my-token");
  });
});
