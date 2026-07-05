import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { deleteFile } from "./deleteFile";

describe("deleteFile", () => {
  it("calls DELETE with the id in the path", async () => {
    // Arrange
    const DELETE = vi.fn().mockResolvedValue({ error: undefined });
    const api = { DELETE } as unknown as ApiClient;

    // Act
    await deleteFile(api, "abc");

    // Assert
    expect(DELETE).toHaveBeenCalledWith("/files/{id}", { params: { path: { id: "abc" } } });
  });

  it("throws when the API returns an error", async () => {
    // Arrange
    const api = { DELETE: vi.fn().mockResolvedValue({ error: { message: "boom" } }) } as unknown as ApiClient;

    // Act + Assert
    await expect(deleteFile(api, "abc")).rejects.toThrow();
  });
});
