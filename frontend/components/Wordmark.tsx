// Wordmark is the Vault brass lock mark next to the name.
export function Wordmark() {
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
