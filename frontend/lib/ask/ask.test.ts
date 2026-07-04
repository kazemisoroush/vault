import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { ask } from "./ask";

describe("ask", () => {
  it("returns the results from the response", async () => {
    // Arrange
    const results = [
      {
        file: { id: "1", name: "a.txt", contentType: "text/plain", size: 1, status: "ready", createdAt: "", updatedAt: "" },
        downloadUrl: "https://get/1",
      },
    ];
    const post = vi.fn().mockResolvedValue({ data: { results } });
    const api = { POST: post } as unknown as ApiClient;

    // Act
    const got = await ask(api, "petrol receipts");

    // Assert
    expect(post).toHaveBeenCalledWith("/ask", { body: { query: "petrol receipts" } });
    expect(got).toEqual(results);
  });

  it("throws when the API returns an error", async () => {
    // Arrange
    const api = {
      POST: vi.fn().mockResolvedValue({ data: undefined, error: { error: "bad" } }),
    } as unknown as ApiClient;

    // Act + Assert
    await expect(ask(api, "x")).rejects.toThrow(/search failed/);
  });
});
