"use client";

import { useEffect, useRef, useState } from "react";

import type { VaultFile } from "../lib/files/vaultFile";

// confirmWindowMs is how long an armed delete stays a red check before reverting to the trash icon.
const confirmWindowMs = 5000;

// FileList shows each file with its extraction status and an inline two-tap delete: the first tap
// arms the control (the trash icon becomes a red check for a few seconds), the second tap deletes.
// If the second tap does not come in time, the control reverts. The control keeps its size either
// way, so the row layout never shifts.
export function FileList({ files, onDelete }: { files: VaultFile[]; onDelete: (id: string) => void }) {
  const [armed, setArmed] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  function disarm() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    setArmed(null);
  }

  function onDeleteClick(id: string) {
    if (armed === id) {
      disarm();
      onDelete(id);
      return;
    }
    if (timer.current) {
      clearTimeout(timer.current);
    }
    setArmed(id);
    timer.current = setTimeout(() => {
      timer.current = null;
      setArmed(null);
    }, confirmWindowMs);
  }

  // Clear the pending revert timer if the list unmounts.
  useEffect(() => () => disarm(), []);

  if (files.length === 0) {
    return <p className="muted">No files yet. Drop one above.</p>;
  }

  return (
    <ul className="files">
      {files.map((file) => {
        const isArmed = armed === file.id;
        return (
          <li key={file.id}>
            <span className="name">{file.name}</span>
            <span className={`badge ${file.status}`}>{file.status}</span>
            <button
              className={isArmed ? "trash armed" : "trash"}
              aria-label={isArmed ? `Confirm delete ${file.name}` : `Delete ${file.name}`}
              onClick={() => onDeleteClick(file.id)}
            >
              {isArmed ? (
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M5 13l4 4L19 7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              ) : (
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2m2 0v12a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V7"
                    stroke="currentColor"
                    strokeWidth="1.6"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              )}
            </button>
          </li>
        );
      })}
    </ul>
  );
}
