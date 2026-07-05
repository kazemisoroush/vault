"use client";

import { useState } from "react";

import type { VaultFile } from "../lib/files/vaultFile";

// FileList shows each file with its extraction status and a delete control.
export function FileList({ files, onDelete }: { files: VaultFile[]; onDelete: (id: string) => void }) {
  const [confirming, setConfirming] = useState<string | null>(null);

  if (files.length === 0) {
    return <p className="muted">No files yet. Drop one above.</p>;
  }

  return (
    <ul className="files">
      {files.map((file) => (
        <li key={file.id}>
          <span className="name">{file.name}</span>
          <span className={`badge ${file.status}`}>{file.status}</span>
          {confirming === file.id ? (
            <span className="rowactions">
              <button
                className="danger"
                onClick={() => {
                  onDelete(file.id);
                  setConfirming(null);
                }}
              >
                Delete
              </button>
              <button className="link" onClick={() => setConfirming(null)}>
                Cancel
              </button>
            </span>
          ) : (
            <button className="trash" aria-label={`Delete ${file.name}`} onClick={() => setConfirming(file.id)}>
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2m2 0v12a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V7"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </button>
          )}
        </li>
      ))}
    </ul>
  );
}
