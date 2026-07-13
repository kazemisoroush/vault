"use client";

import { MODE_STORAGE_KEY, type Mode } from "../lib/mode";

// ModeToggle flips the app between its two faces: the personal vault and the Cited legal view.
// The mode is stamped on the document element (the palette swap lives in CSS) and persisted;
// the page owns the state so the rest of the view can follow it.
export function ModeToggle({ mode, onMode }: { mode: Mode; onMode: (mode: Mode) => void }) {
  function set(next: Mode) {
    if (next === mode) return;
    document.documentElement.dataset.mode = next;
    try {
      localStorage.setItem(MODE_STORAGE_KEY, next);
    } catch {
      // ignore storage failures
    }
    onMode(next);
  }

  return (
    <div className="modetoggle" role="group" aria-label="Switch app mode">
      <button className={mode === "personal" ? "on" : ""} onClick={() => set("personal")}>
        Personal
      </button>
      <button className={mode === "legal" ? "on" : ""} onClick={() => set("legal")}>
        Cited
      </button>
    </div>
  );
}
