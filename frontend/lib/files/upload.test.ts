import { afterEach, describe, expect, it, vi } from "vitest";

import { uploadBytes } from "./upload";

describe("uploadBytes", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("PUTs the file to the url with its content type", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchMock);
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    await uploadBytes("https://s3/put", file, "text/plain");

    expect(fetchMock).toHaveBeenCalledWith("https://s3/put", {
      method: "PUT",
      headers: { "Content-Type": "text/plain" },
      body: file,
    });
  });

  it("throws when S3 rejects the upload", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 403 }));
    const file = new File(["hi"], "a.txt", { type: "text/plain" });

    await expect(uploadBytes("https://s3/put", file, "text/plain")).rejects.toThrow(/403/);
  });
});
