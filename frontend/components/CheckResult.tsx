"use client";

import { useState } from "react";

import type { Check, Claim } from "../lib/checks/check";
import { claimSegments } from "../lib/checks/claimSegments";

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

// CheckResult renders a finished check where an ask reply would render: highlighted sentences,
// references unfolding inline under the clicked sentence, and the verdict tally.
export function CheckResult({ check, onReset }: { check: Check; onReset: () => void }) {
  const [open, setOpen] = useState<number | null>(null);
  const claims = check.claims ?? [];

  return (
    <div className="panel check-result">
      <p className="eyebrow">Checked against your documents</p>
      <p className="check-text">
        {claimSegments(check.text, claims).map((segment, i) => {
          if (segment.claimIndex === undefined) {
            return <span key={i}>{segment.text}</span>;
          }
          const index = segment.claimIndex;
          return (
            <span key={i}>
              <button
                type="button"
                className={`claim ${claims[index].verdict}${open === index ? " open" : ""}`}
                onClick={() => setOpen(open === index ? null : index)}
              >
                {segment.text}
              </button>
              {open === index && <InlineReferences claim={claims[index]} />}
            </span>
          );
        })}
      </p>
      <p className="verdict-rail">
        {(["verified", "disputed", "review", "unsupported"] as const)
          .map((verdict) => ({ verdict, count: claims.filter((c) => c.verdict === verdict).length }))
          .filter(({ count }) => count > 0)
          .map(({ verdict, count }) => (
            <span key={verdict} className={verdict}>
              {count} {verdict}
            </span>
          ))}
      </p>
      <button className="btn another" type="button" onClick={onReset}>
        Check another
      </button>
    </div>
  );
}

// InlineReferences unfolds one claim's gate-verified passages directly under its sentence.
function InlineReferences({ claim }: { claim: Claim }) {
  const references = claim.references ?? [];
  return (
    <span className="inline-refs">
      <span className={`refs-verdict ${claim.verdict}`}>{verdictLine(claim.verdict)}</span>
      {references.length === 0 ? (
        <span className="empty">
          The search found no passage for this sentence. That certifies nothing either way;
          silence is where to look hardest.
        </span>
      ) : (
        references.map((reference, i) => (
          <span key={i} className="ref-card">
            <span className="ref-head">
              <span className={`rel ${reference.relation}`}>{reference.relation}</span>
              <span className="file">{reference.fileName}</span>
            </span>
            <span className="ref-span">“{reference.spanText}”</span>
          </span>
        ))
      )}
    </span>
  );
}
