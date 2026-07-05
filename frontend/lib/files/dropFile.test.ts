import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import type { ContentHasher } from "./contentHasher";
import { dropFile } from "./dropFile";
import { uploadBytes } from "./upload";

vi.mock("./upload", () => ({ uploadBytes: vi.fn() }));

const record = {
  id: "h",
  name: "a.txt",
  contentType: "text/plain",
  size: 2,
  status: "pending",
  createdAt: "",
  updatedAt: "",
};

const hasher: ContentHasher = { hash: vi.fn().mockResolvedValue("hash123") };

describe("dropFile", () => {
  beforeEach(() => vi.mocked(uploadBytes).mockReset());

  it("hashes, registers with the hash, then uploads a new file", async () => {
    // Arrange
    const post = vi.fn().mockResolvedValue({ data: { file: record, uploadUrl: "https://s3/put" } });
    const api = { POST: post } as unknown as ApiClient;
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    // Act
    const result = await dropFile(api, file, hasher);

    // Assert
    expect(post).toHaveBeenCalledWith("/files", {
      body: { name: "a.txt", contentType: "text/plain", size: 2, hash: "hash123" },
    });
    expect(uploadBytes).toHaveBeenCalledWith("https://s3/put", file, "text/plain");
    expect(result).toBe(record);
  });

  it("skips the upload for a duplicate (no uploadUrl)", async () => {
    // Arrange: the file already exists, so the server returns no upload URL.
    const post = vi.fn().mockResolvedValue({ data: { file: record } });
    const api = { POST: post } as unknown as ApiClient;
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    // Act
    const result = await dropFile(api, file, hasher);

    // Assert
    expect(uploadBytes).not.toHaveBeenCalled();
    expect(result).toBe(record);
  });

  it("throws and skips the upload when the API returns an error", async () => {
    // Arrange
    const post = vi.fn().mockResolvedValue({ data: undefined, error: { error: "bad" } });
    const api = { POST: post } as unknown as ApiClient;
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    // Act + Assert
    await expect(dropFile(api, file, hasher)).rejects.toThrow(/could not register/);
    expect(uploadBytes).not.toHaveBeenCalled();
  });
});
