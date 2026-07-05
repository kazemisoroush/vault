import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { updateFile } from "./updateFile";

describe("updateFile", () => {
  it("PATCHes the new name at the id", async () => {
    // Arrange
    const PATCH = vi.fn().mockResolvedValue({ error: undefined });
    const api = { PATCH } as unknown as ApiClient;

    // Act
    await updateFile(api, "abc", "new.txt");

    // Assert
    expect(PATCH).toHaveBeenCalledWith("/files/{id}", { params: { path: { id: "abc" } }, body: { name: "new.txt" } });
  });

  it("throws when the API returns an error", async () => {
    // Arrange
    const api = { PATCH: vi.fn().mockResolvedValue({ error: { message: "boom" } }) } as unknown as ApiClient;

    // Act + Assert
    await expect(updateFile(api, "abc", "new.txt")).rejects.toThrow();
  });
});
