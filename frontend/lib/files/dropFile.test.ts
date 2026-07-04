import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { dropFile } from "./dropFile";
import { uploadBytes } from "./upload";

vi.mock("./upload", () => ({ uploadBytes: vi.fn() }));

const record = {
  id: "1",
  name: "a.txt",
  contentType: "text/plain",
  size: 2,
  status: "pending",
  createdAt: "",
  updatedAt: "",
};

describe("dropFile", () => {
  beforeEach(() => vi.mocked(uploadBytes).mockReset());

  it("registers the record then uploads the bytes", async () => {
    // Arrange
    const post = vi.fn().mockResolvedValue({ data: { file: record, uploadUrl: "https://s3/put" } });
    const api = { POST: post } as unknown as ApiClient;
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    // Act
    const result = await dropFile(api, file);

    // Assert
    expect(post).toHaveBeenCalledWith("/files", {
      body: { name: "a.txt", contentType: "text/plain", size: 2 },
    });
    expect(uploadBytes).toHaveBeenCalledWith("https://s3/put", file, "text/plain");
    expect(result).toBe(record);
  });

  it("throws and skips the upload when the API returns an error", async () => {
    // Arrange
    const post = vi.fn().mockResolvedValue({ data: undefined, error: { error: "bad" } });
    const api = { POST: post } as unknown as ApiClient;
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    // Act + Assert
    await expect(dropFile(api, file)).rejects.toThrow(/could not register/);
    expect(uploadBytes).not.toHaveBeenCalled();
  });

  it("defaults an unknown content type", async () => {
    // Arrange
    const post = vi.fn().mockResolvedValue({ data: { file: record, uploadUrl: "u" } });
    const api = { POST: post } as unknown as ApiClient;
    const file = new File([""], "blob", { type: "" });

    // Act
    await dropFile(api, file);

    // Assert
    expect(post).toHaveBeenCalledWith("/files", {
      body: { name: "blob", contentType: "application/octet-stream", size: 0 },
    });
    expect(uploadBytes).toHaveBeenCalledWith("u", file, "application/octet-stream");
  });
});
