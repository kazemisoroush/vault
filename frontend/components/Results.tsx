import type { AskResult } from "../lib/ask/askResult";

// Results lists the matched files, each opening through its presigned download URL.
export function Results({ results }: { results: AskResult[] }) {
  if (results.length === 0) {
    return <p className="muted">No matches.</p>;
  }

  return (
    <ul className="results">
      {results.map(({ file, downloadUrl }) => (
        <li key={file.id}>
          <a href={downloadUrl} target="_blank" rel="noopener noreferrer">
            {file.name}
          </a>
        </li>
      ))}
    </ul>
  );
}
