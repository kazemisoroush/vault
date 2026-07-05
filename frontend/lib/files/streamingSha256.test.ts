import { describe, expect, it } from "vitest";

import { StreamingSha256 } from "./streamingSha256";

describe("StreamingSha256", () => {
  it("hashes a file to its SHA-256 hex", async () => {
    // Arrange: the SHA-256 of "abc" is a known constant.
    const file = new File(["abc"], "a.txt");

    // Act
    const hash = await new StreamingSha256().hash(file);

    // Assert
    expect(hash).toBe("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
  });

  it("hashes an empty file to the empty SHA-256", async () => {
    // Arrange
    const file = new File([], "empty");

    // Act + Assert
    expect(await new StreamingSha256().hash(file)).toBe(
      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    );
  });
});
