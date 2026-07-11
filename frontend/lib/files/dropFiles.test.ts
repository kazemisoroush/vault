import { beforeEach, describe, expect, it, vi } from "vitest";

import { dropFiles } from "./dropFiles";
import { dropFile } from "./dropFile";
import type { ApiClient } from "../api/client";

vi.mock("./dropFile", () => ({ dropFile: vi.fn() }));

const mockDropFile = vi.mocked(dropFile);
const api = {} as ApiClient;

function fileNamed(name: string): File {
  return new File(["x"], name);
}

describe("dropFiles", () => {
  beforeEach(() => {
    mockDropFile.mockReset();
  });

  it("uploads every file and reports progress to the end", async () => {
    // Arrange
    mockDropFile.mockImplementation(async (_api, file) => ({ id: file.name, name: file.name }) as never);
    const seen: number[] = [];

    // Act
    const result = await dropFiles(api, [fileNamed("a"), fileNamed("b"), fileNamed("c")], (p) => seen.push(p.done));

    // Assert
    expect(result.uploaded).toHaveLength(3);
    expect(result.failed).toHaveLength(0);
    expect(seen.at(-1)).toBe(3);
    expect(mockDropFile).toHaveBeenCalledTimes(3);
  });

  it("continues past a failing file and collects it", async () => {
    // Arrange: the middle file fails.
    mockDropFile.mockImplementation(async (_api, file) => {
      if (file.name === "bad") throw new Error("boom");
      return { id: file.name, name: file.name } as never;
    });

    // Act
    const result = await dropFiles(api, [fileNamed("a"), fileNamed("bad"), fileNamed("c")]);

    // Assert: the batch still lands the good files, and the bad one is reported.
    expect(result.uploaded).toHaveLength(2);
    expect(result.failed).toHaveLength(1);
    expect(result.failed[0].file.name).toBe("bad");
  });
});
