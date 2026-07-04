import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { listCalls } from "./listCalls";

describe("listCalls", () => {
  it("returns the calls from the response", async () => {
    // Arrange
    const calls = [
      { op: "retrieve", model: "haiku", latencyMs: 10, inputTokens: 1, outputTokens: 2, ok: true, createdAt: "2026-01-01T00:00:00Z" },
    ];
    const api = { GET: vi.fn().mockResolvedValue({ data: { calls } }) } as unknown as ApiClient;

    // Act
    const got = await listCalls(api);

    // Assert
    expect(got).toEqual(calls);
  });

  it("returns an empty array when the response has no calls", async () => {
    // Arrange
    const api = { GET: vi.fn().mockResolvedValue({ data: undefined }) } as unknown as ApiClient;

    // Act + Assert
    expect(await listCalls(api)).toEqual([]);
  });
});
