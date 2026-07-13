"use client";

import type { Claim } from "../lib/checks/check";
import type { VaultFile } from "../lib/files/vaultFile";
import { References } from "./References";

// RecordPanel lists the record, or the selected claim's references.
export function RecordPanel({
  files,
  claim,
  onBack,
}: {
  files: VaultFile[];
  claim?: Claim;
  onBack: () => void;
}) {
  return (
    <section className="record" aria-label="The record">
      {claim === undefined ? (
        <>
          <p className="eyebrow">The record · {files.length === 1 ? "1 document" : `${files.length} documents`}</p>
          {files.length === 0 && <p className="empty">Nothing here yet. Drop documents in Personal mode first.</p>}
          <ul className="record-list">
            {files.map((file) => (
              <li key={file.id} className="record-doc">
                <span className={`dot ${file.status}`} aria-hidden="true" />
                <span className="name">{file.name}</span>
                <span className="state">{file.status}</span>
              </li>
            ))}
          </ul>
        </>
      ) : (
        <References claim={claim} onBack={onBack} />
      )}
    </section>
  );
}
