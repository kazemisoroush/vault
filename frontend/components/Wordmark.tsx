import type { Mode } from "../lib/mode";

// Wordmark is the mark next to the name: the brass lock for the vault, the scales for Cited.
export function Wordmark({ mode = "personal" }: { mode?: Mode }) {
  if (mode === "legal") {
    return (
      <span className="wordmark">
        <svg className="mark" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M12 4v16M7 20h10" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          <path d="M5 7h14" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          <path
            d="M7 7l-2.6 5a2.8 2.8 0 0 0 5.2 0L7 7ZM17 7l-2.6 5a2.8 2.8 0 0 0 5.2 0L17 7Z"
            stroke="currentColor"
            strokeWidth="1.4"
            strokeLinejoin="round"
          />
        </svg>
        Cited
      </span>
    );
  }
  return (
    <span className="wordmark">
      <svg className="mark" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <rect x="3" y="3" width="18" height="18" rx="5" stroke="currentColor" strokeWidth="1.6" />
        <circle cx="12" cy="10.5" r="2.4" stroke="currentColor" strokeWidth="1.6" />
        <path d="M12 12.6v4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
      </svg>
      Vault
    </span>
  );
}
