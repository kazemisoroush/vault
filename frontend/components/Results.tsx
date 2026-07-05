import type { AskResult } from "../lib/ask/askResult";

// extension returns a short uppercase file-type tag for the chip.
function extension(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1, dot + 5).toUpperCase() : "FILE";
}

// Results lists the matched files, each opening through its presigned download URL.
export function Results({ results }: { results: AskResult[] }) {
  if (results.length === 0) {
    return <p className="muted">No matches.</p>;
  }

  return (
    <ul className="results">
      {results.map(({ file, downloadUrl }) => (
        <li className="result" key={file.id}>
          <span className="filetype">{extension(file.name)}</span>
          <span className="body">
            <a href={downloadUrl} target="_blank" rel="noopener noreferrer">
              {file.name}
            </a>
            <div className="why">{file.contentType}</div>
          </span>
        </li>
      ))}
    </ul>
  );
}
