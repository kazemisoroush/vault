"use client";

import { useState, type FormEvent } from "react";

// CheckBox is the paste field for verification, the Cited twin of AskBox.
export function CheckBox({ onCheck, busy }: { onCheck: (text: string) => void; busy: boolean }) {
  const [text, setText] = useState("");

  function submit(event: FormEvent) {
    event.preventDefault();
    const trimmed = text.trim();
    if (trimmed) onCheck(trimmed);
  }

  return (
    <form className="ask stacked" onSubmit={submit}>
      <textarea
        value={text}
        onChange={(event) => setText(event.target.value)}
        placeholder="Paste the text to verify against your documents…"
        aria-label="Text to check"
        rows={6}
      />
      <button className="btn" type="submit" disabled={busy || text.trim() === ""}>
        {busy ? "Starting…" : "Check"}
      </button>
    </form>
  );
}
