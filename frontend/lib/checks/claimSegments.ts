import type { Claim } from "./check";

// Segment is one run of the checked text: either plain text between claims, or a claim with its
// index into the check's claims array.
export type Segment = {
  text: string;
  claimIndex?: number;
};

// claimSegments cuts the checked text into renderable runs using the contract's UTF-8 BYTE
// offsets. The text is encoded once, sliced by bytes, and decoded per run: indexing the
// JavaScript string directly would drift on any multibyte character, and a drifted highlight
// would silently mislabel a sentence.
export function claimSegments(text: string, claims: Claim[]): Segment[] {
  const bytes = new TextEncoder().encode(text);
  const decoder = new TextDecoder();
  const ordered = claims
    .map((claim, claimIndex) => ({ claim, claimIndex }))
    .sort((a, b) => a.claim.start - b.claim.start);

  const segments: Segment[] = [];
  let cursor = 0;
  for (const { claim, claimIndex } of ordered) {
    // A claim outside the byte range or overlapping the previous one cannot be rendered
    // honestly, so it is skipped rather than guessed at.
    if (claim.start < cursor || claim.end > bytes.length || claim.end <= claim.start) {
      continue;
    }
    if (claim.start > cursor) {
      segments.push({ text: decoder.decode(bytes.subarray(cursor, claim.start)) });
    }
    segments.push({ text: decoder.decode(bytes.subarray(claim.start, claim.end)), claimIndex });
    cursor = claim.end;
  }
  if (cursor < bytes.length) {
    segments.push({ text: decoder.decode(bytes.subarray(cursor)) });
  }
  return segments;
}
