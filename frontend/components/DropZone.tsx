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
      <p>{busy ? "Uploading…" : "Drop a file here, or click to pick one"}</p>
    </div>
  );
}
