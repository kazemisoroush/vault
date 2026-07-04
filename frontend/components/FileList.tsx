import type { VaultFile } from "../lib/files/vaultFile";

// FileList shows each file with its extraction status.
export function FileList({ files }: { files: VaultFile[] }) {
  if (files.length === 0) {
    return <p className="muted">No files yet. Drop one above.</p>;
  }

  return (
    <ul className="files">
      {files.map((file) => (
        <li key={file.id}>
          <span className="name">{file.name}</span>
          <span className={`status ${file.status}`}>{file.status}</span>
        </li>
      ))}
    </ul>
  );
}
