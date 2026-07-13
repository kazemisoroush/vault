"use client";

import { useState, type FormEvent } from "react";

import type { Check } from "../lib/checks/check";
import { claimSegments } from "../lib/checks/claimSegments";

// verdictCounts tallies how many claims landed on each verdict.
function verdictCounts(check: Check): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const claim of check.claims ?? []) {
    counts[claim.verdict] = (counts[claim.verdict] ?? 0) + 1;
  }
  return counts;
}

// DraftPanel is the paste-and-check side of the Cited view.
export function DraftPanel({
  check,
  submitting,
  selected,
  onCheck,
  onSelect,
  onReset,
}: {
  check: Check | null;
  submitting: boolean;
  selected: number | null;
  onCheck: (text: string) => void;
  onSelect: (index: number) => void;
  onReset: () => void;
}) {
  const [text, setText] = useState("");

  function submit(event: FormEvent) {
    event.preventDefault();
    const trimmed = text.trim();
    if (trimmed) onCheck(trimmed);
  }

  const running = check !== null && (check.status === "pending" || check.status === "running");

  return (
    <section className="draft" aria-label="The draft">
      <p className="eyebrow">The draft</p>

      {check === null && (
        <form className="draft-form" onSubmit={submit}>
          <textarea
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="Paste the text to verify against your documents…"
            aria-label="Text to check"
            rows={10}
          />
          <button className="btn" type="submit" disabled={submitting || text.trim() === ""}>
            {submitting ? "Starting…" : "Check it"}
          </button>
        </form>
      )}

      {running && (
        <p className="draft-status">Checking… every sentence is being matched against your documents.</p>
      )}

      {check !== null && check.status === "failed" && (
        <>
          <p role="alert">This check failed to finish. Try a shorter text, or try again.</p>
          <button className="btn" type="button" onClick={onReset}>
            Check another
          </button>
        </>
      )}

      {check !== null && check.status === "done" && (
        <>
          <VerdictRail check={check} />
          <p className="draft-text">
            {claimSegments(check.text, check.claims ?? []).map((segment, i) =>
              segment.claimIndex === undefined ? (
                <span key={i}>{segment.text}</span>
              ) : (
                <button
                  key={i}
                  type="button"
                  className={`claim ${(check.claims ?? [])[segment.claimIndex].verdict}${
                    selected === segment.claimIndex ? " sel" : ""
                  }`}
                  onClick={() => onSelect(segment.claimIndex as number)}
                >
                  {segment.text}
                </button>
              ),
            )}
          </p>
          <p className="legend">
            <span className="verified">verified</span>
            <span className="disputed">disputed</span>
            <span className="review">review</span>
            <span className="unsupported">unsupported</span>
          </p>
          <button className="btn another" type="button" onClick={onReset}>
            Check another
          </button>
        </>
      )}
    </section>
  );
}

// VerdictRail is the tally line above a finished check.
function VerdictRail({ check }: { check: Check }) {
  const counts = verdictCounts(check);
  const order = ["verified", "disputed", "review", "unsupported"];
  return (
    <p className="verdict-rail">
      {order
        .filter((verdict) => counts[verdict])
        .map((verdict) => (
          <span key={verdict} className={verdict}>
            {counts[verdict]} {verdict}
          </span>
        ))}
    </p>
  );
}
