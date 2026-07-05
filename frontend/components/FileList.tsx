"use client";

import { useState } from "react";

import type { VaultFile } from "../lib/files/vaultFile";

type FileListProps = {
  files: VaultFile[];
  onDelete: (id: string) => void;
  onRename: (id: string, name: string) => void;
};

// FileList shows each file with its extraction status, a rename control, and a delete control.
export function FileList({ files, onDelete, onRename }: FileListProps) {
  const [confirming, setConfirming] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");

  if (files.length === 0) {
    return <p className="muted">No files yet. Drop one above.</p>;
  }

  return (
    <ul className="files">
      {files.map((file) => (
        <li key={file.id}>
          {editing === file.id ? (
            <form
              className="rename"
              onSubmit={(event) => {
                event.preventDefault();
                const next = draft.trim();
                if (next && next !== file.name) {
                  onRename(file.id, next);
                }
                setEditing(null);
              }}
            >
              <input
                autoFocus
                className="renameinput"
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                aria-label={`New name for ${file.name}`}
              />
              <button className="save" type="submit">
                Save
              </button>
              <button className="link" type="button" onClick={() => setEditing(null)}>
                Cancel
              </button>
            </form>
          ) : (
            <>
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
                <span className="rowactions">
                  <button
                    className="rowicon"
                    aria-label={`Rename ${file.name}`}
                    onClick={() => {
                      setDraft(file.name);
                      setEditing(file.id);
                    }}
                  >
                    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                      <path
                        d="M4 20h4L18.5 9.5a2 2 0 0 0-2.83-2.83L5 17.2V20zM13.5 6.5l4 4"
                        stroke="currentColor"
                        strokeWidth="1.6"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                  <button className="rowicon trash" aria-label={`Delete ${file.name}`} onClick={() => setConfirming(file.id)}>
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
                </span>
              )}
            </>
          )}
        </li>
      ))}
    </ul>
  );
}
