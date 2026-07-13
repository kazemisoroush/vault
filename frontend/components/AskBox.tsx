"use client";

import { useState, type FormEvent } from "react";

// AskBox is the single text input for natural-language retrieval.
export function AskBox({ onAsk, busy }: { onAsk: (query: string) => void; busy: boolean }) {
  const [query, setQuery] = useState("");

  function submit(event: FormEvent) {
    event.preventDefault();
    const trimmed = query.trim();
    if (trimmed) onAsk(trimmed);
  }

  return (
    <form className="ask stacked" onSubmit={submit}>
      <span className="field">
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="1.7" />
          <path d="m20 20-3.2-3.2" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
        </svg>
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Ask for anything… petrol receipts from last month"
          aria-label="Ask for a file"
        />
      </span>
      <button className="btn" type="submit" disabled={busy}>
        {busy ? "Searching…" : "Ask"}
      </button>
    </form>
  );
}
