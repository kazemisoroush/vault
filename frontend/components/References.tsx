"use client";

import type { Claim } from "../lib/checks/check";

// verdictLine says what the claim's verdict may honestly promise.
function verdictLine(verdict: Claim["verdict"]): string {
  switch (verdict) {
    case "verified":
      return "verified: code-proven quote";
    case "disputed":
      return "disputed: the record disagrees; you decide";
    case "review":
      return "review: confirm the passage supports your wording";
    default:
      return "unsupported: no supporting passage was confirmed";
  }
}

// References shows every gate-verified passage bearing on one claim.
export function References({ claim, onBack }: { claim: Claim; onBack: () => void }) {
  const references = claim.references ?? [];
  return (
    <div className="refs">
      <button className="ghost back" onClick={onBack}>
        ← The record
      </button>
      <p className={`refs-verdict ${claim.verdict}`}>{verdictLine(claim.verdict)}</p>
      <blockquote className="refs-claim">{claim.text}</blockquote>
      {references.length === 0 ? (
        <p className="empty">
          The search found no passage for this sentence. That certifies nothing either way;
          silence is where to look hardest.
        </p>
      ) : (
        <ul className="refs-list">
          {references.map((reference, i) => (
            <li key={i} className="ref-card">
              <p className="ref-head">
                <span className={`rel ${reference.relation}`}>{reference.relation}</span>
                <span className="file">{reference.fileName}</span>
              </p>
              <blockquote className="ref-span">“{reference.spanText}”</blockquote>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
