import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { listFiles } from "./listFiles";

describe("listFiles", () => {
  it("returns the files from the response", async () => {
    // Arrange
    const files = [
      { id: "1", name: "a.txt", contentType: "text/plain", size: 1, status: "ingested", createdAt: "", updatedAt: "" },
    ];
    const api = { GET: vi.fn().mockResolvedValue({ data: { files } }) } as unknown as ApiClient;

    // Act
    const result = await listFiles(api);

    // Assert
    expect(result).toEqual(files);
  });

  it("returns an empty array when the response has no files", async () => {
    // Arrange
    const api = { GET: vi.fn().mockResolvedValue({ data: undefined }) } as unknown as ApiClient;

    // Act
    const result = await listFiles(api);

    // Assert
    expect(result).toEqual([]);
  });
});
