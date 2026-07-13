import { describe, expect, it } from "vitest";

import { claimSegments } from "./claimSegments";
import type { Claim } from "./check";

// byteOffsets locates a substring's UTF-8 byte range the same way the backend does, so the
// tests exercise real contract offsets rather than hand-counted ones.
function byteOffsets(text: string, part: string): { start: number; end: number } {
  const encoder = new TextEncoder();
  const all = encoder.encode(text);
  const target = encoder.encode(part);
  outer: for (let i = 0; i <= all.length - target.length; i++) {
    for (let j = 0; j < target.length; j++) {
      if (all[i + j] !== target[j]) continue outer;
    }
    return { start: i, end: i + target.length };
  }
  throw new Error(`${part} not in text`);
}

function claimOf(text: string, part: string, verdict = "unsupported"): Claim {
  const { start, end } = byteOffsets(text, part);
  return { text: part, start, end, verdict: verdict as Claim["verdict"] };
}

describe("claimSegments", () => {
  it("slices claims and gaps so they reassemble to the original text", () => {
    // Arrange
    const text = "Heading\nThe deposit was paid. The keys were returned.";
    const claims = [claimOf(text, "The deposit was paid."), claimOf(text, "The keys were returned.")];

    // Act
    const segments = claimSegments(text, claims);

    // Assert
    expect(segments.map((s) => s.text).join("")).toBe(text);
    expect(segments.filter((s) => s.claimIndex !== undefined)).toHaveLength(2);
  });

  it("keeps byte offsets honest across multibyte characters", () => {
    // Arrange: the curly quotes are multibyte; naive string indexing would drift.
    const text = "He wrote “the funds cleared.” Then he resigned.";
    const claims = [claimOf(text, "Then he resigned.")];

    // Act
    const segments = claimSegments(text, claims);

    // Assert
    const highlighted = segments.find((s) => s.claimIndex === 0);
    expect(highlighted?.text).toBe("Then he resigned.");
    expect(segments.map((s) => s.text).join("")).toBe(text);
  });

  it("skips a claim with impossible offsets rather than guessing", () => {
    // Arrange
    const text = "Short text.";
    const broken: Claim = { text: "beyond", start: 5, end: 999, verdict: "unsupported" };

    // Act
    const segments = claimSegments(text, [broken]);

    // Assert: the text still renders whole, with no highlight.
    expect(segments.map((s) => s.text).join("")).toBe(text);
    expect(segments.every((s) => s.claimIndex === undefined)).toBe(true);
  });

  it("renders out-of-order claims in text order", () => {
    // Arrange
    const text = "First point. Second point.";
    const claims = [claimOf(text, "Second point."), claimOf(text, "First point.")];

    // Act
    const segments = claimSegments(text, claims);

    // Assert: claimIndex still points at the original claims array.
    const marked = segments.filter((s) => s.claimIndex !== undefined);
    expect(marked[0].text).toBe("First point.");
    expect(marked[0].claimIndex).toBe(1);
    expect(marked[1].claimIndex).toBe(0);
  });
});
