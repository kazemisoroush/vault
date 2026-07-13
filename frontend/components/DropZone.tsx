"use client";

import { useEffect, useRef, useState, type DragEvent, type KeyboardEvent } from "react";

import { collectFiles, dropEntries, filterFiles } from "../lib/files/collectFiles";

// DropZone accepts files or whole folders, by drag-and-drop or by picker, and hands the flattened
// list to onFiles. A dropped folder is walked into its files; storage stays flat. While busy it
// shows a spinner, and it shows a brief reading state while a large folder is being walked.
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
  const folderRef = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const [reading, setReading] = useState(false);
  const active = busy || reading;

  // webkitdirectory is not a typed React attribute, so set it on the folder picker after mount.
  useEffect(() => {
    folderRef.current?.setAttribute("webkitdirectory", "");
  }, []);

  function open() {
    if (active) return;
    inputRef.current?.click();
  }

  function openFolder() {
    if (active) return;
    folderRef.current?.click();
  }

  // hand feeds a flat picked list (files or a folder's files) through the shared skip rules.
  function hand(list: FileList | File[] | null) {
    const files = filterFiles(Array.from(list ?? []));
    if (files.length > 0) onFiles(files);
  }

  async function handleDrop(event: DragEvent) {
    event.preventDefault();
    setOver(false);
    if (active) return;

    // Read the entries and a flat-file snapshot from the event synchronously; both expire once the
    // event returns, so they cannot be read after the await below.
    const entries = dropEntries(event.dataTransfer.items);
    const flat = Array.from(event.dataTransfer.files);

    if (entries.length === 0) {
      // No entries API available: fall back to the flat file list, which cannot see into folders.
      hand(flat);
      return;
    }

    setReading(true);
    try {
      const files = await collectFiles(entries);
      if (files.length > 0) onFiles(files);
    } catch {
      // The walk failed outright; fall back to the flat files the drop also carried.
      hand(flat);
    } finally {
      setReading(false);
    }
  }

  function handleKey(event: KeyboardEvent) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  }

  const label = reading
    ? "Reading folder…"
    : busy
      ? pending && pending > 0
        ? `Uploading ${pending}…`
        : "Uploading…"
      : "Drop files or a folder here";

  return (
    <div
      className={over ? "dropzone over" : "dropzone"}
      role="button"
      tabIndex={0}
      aria-label="Add files or a folder to the vault"
      aria-disabled={active}
      aria-busy={active}
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
      <input
        ref={folderRef}
        type="file"
        hidden
        onChange={(event) => {
          hand(event.target.files);
          event.target.value = "";
        }}
      />
      {active ? (
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
      <span className="hint">
        click to choose, or{" "}
        <button
          type="button"
          className="folderpick"
          disabled={active}
          onClick={(event) => {
            event.stopPropagation();
            openFolder();
          }}
        >
          add a folder
        </button>{" "}
        · the vault reads them for you
      </span>
    </div>
  );
}
