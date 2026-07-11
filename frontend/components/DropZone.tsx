"use client";

import { useRef, useState, type DragEvent, type KeyboardEvent } from "react";

// DropZone accepts one or many files by drag-and-drop or click, and hands them to onFiles. While
// busy it shows a spinner and, when known, how many files are still uploading.
export function DropZone({
  onFiles,
  busy,
  pending,
}: {
  onFiles: (files: File[]) => void;
  busy: boolean;
  pending?: number;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);

  function open() {
    if (busy) return;
    inputRef.current?.click();
  }

  function hand(list: FileList | null) {
    const files = Array.from(list ?? []);
    if (files.length > 0) onFiles(files);
  }

  function handleDrop(event: DragEvent) {
    event.preventDefault();
    setOver(false);
    if (busy) return;
    hand(event.dataTransfer.files);
  }

  function handleKey(event: KeyboardEvent) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  }

  const label = busy ? (pending && pending > 0 ? `Uploading ${pending}…` : "Uploading…") : "Drop files here";

  return (
    <div
      className={over ? "dropzone over" : "dropzone"}
      role="button"
      tabIndex={0}
      aria-label="Add files to the vault"
      aria-disabled={busy}
      aria-busy={busy}
      onClick={open}
      onKeyDown={handleKey}
      onDragOver={(event) => {
        event.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={handleDrop}
    >
      <input
        ref={inputRef}
        type="file"
        multiple
        hidden
        onChange={(event) => {
          hand(event.target.files);
          event.target.value = "";
        }}
      />
      {busy ? (
        <svg className="spinner" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="2" strokeOpacity="0.25" />
          <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path
            d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path d="M5 15v3a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-3" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
        </svg>
      )}
      <strong>{label}</strong>
      <span className="hint">or click to choose · drop several at once · the vault reads them for you</span>
    </div>
  );
}
