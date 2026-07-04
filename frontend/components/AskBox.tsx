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
    <form className="ask" onSubmit={submit}>
      <input
        type="search"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Ask for anything in your vault"
        aria-label="Ask for a file"
      />
      <button type="submit" disabled={busy}>
        {busy ? "Searching…" : "Ask"}
      </button>
    </form>
  );
}
