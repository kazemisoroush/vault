import type { Claim } from "./check";

// Segment is one renderable run of the checked text.
export type Segment = {
  text: string;
  claimIndex?: number;
};

// claimSegments cuts the checked text into runs by the contract's UTF-8 byte offsets.
export function claimSegments(text: string, claims: Claim[]): Segment[] {
  const bytes = new TextEncoder().encode(text);
  const decoder = new TextDecoder();
  const ordered = claims
    .map((claim, claimIndex) => ({ claim, claimIndex }))
    .sort((a, b) => a.claim.start - b.claim.start);

  const segments: Segment[] = [];
  let cursor = 0;
  for (const { claim, claimIndex } of ordered) {
    // A claim outside the byte range, overlapping the previous one, or cutting a character in
    // half cannot be rendered honestly, so it is skipped rather than guessed at.
    if (
      claim.start < cursor ||
      claim.end > bytes.length ||
      claim.end <= claim.start ||
      isContinuationByte(bytes[claim.start]) ||
      isContinuationByte(bytes[claim.end])
    ) {
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

// isContinuationByte reports whether a byte sits inside a multibyte UTF-8 character.
function isContinuationByte(byte: number | undefined): boolean {
  return byte !== undefined && (byte & 0xc0) === 0x80;
}
