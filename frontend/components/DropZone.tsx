"use client";

import { useRef, useState, type DragEvent, type KeyboardEvent } from "react";

// DropZone accepts a single file by drag-and-drop or click, and hands it to onFile.
export function DropZone({ onFile, busy }: { onFile: (file: File) => void; busy: boolean }) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);

  function open() {
    if (busy) return;
    inputRef.current?.click();
  }

  function handleDrop(event: DragEvent) {
    event.preventDefault();
    setOver(false);
    if (busy) return;
    const file = event.dataTransfer.files?.[0];
    if (file) onFile(file);
  }

  function handleKey(event: KeyboardEvent) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  }

  return (
    <div
      className={over ? "dropzone over" : "dropzone"}
      role="button"
      tabIndex={0}
      aria-label="Add a file to the vault"
      aria-disabled={busy}
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
        hidden
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) onFile(file);
          event.target.value = "";
        }}
      />
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
      <strong>{busy ? "Uploading…" : "Drop a file here"}</strong>
      <span className="hint">or click to choose · no forms, the vault reads it for you</span>
    </div>
  );
}
