import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../api/client";
import { reportTimeToFile } from "./reportTimeToFile";

describe("reportTimeToFile", () => {
  it("posts the measured milliseconds", async () => {
    // Arrange
    const POST = vi.fn().mockResolvedValue({ error: undefined });
    const api = { POST } as unknown as ApiClient;

    // Act
    await reportTimeToFile(api, 1500);

    // Assert
    expect(POST).toHaveBeenCalledWith("/metrics/time-to-file", { body: { ms: 1500 } });
  });

  it("never throws when the request fails", async () => {
    // Arrange
    const api = { POST: vi.fn().mockRejectedValue(new Error("network")) } as unknown as ApiClient;

    // Act + Assert
    await expect(reportTimeToFile(api, 1500)).resolves.toBeUndefined();
  });
});
