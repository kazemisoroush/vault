"use client";

import { useState } from "react";

import type { AskOutcome } from "../lib/ask/askOutcome";
import type { AskResult } from "../lib/ask/askResult";

// extension returns a short uppercase file-type tag for the source chip.
function extension(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1, dot + 5).toUpperCase() : "FILE";
}

// Source is one grounded file: a chip with its type, its name, and a tap-through to open it.
function Source({ file, downloadUrl }: AskResult) {
  return (
    <a
      className="source"
      href={downloadUrl}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Open ${file.name}`}
    >
      <span className="tag">{extension(file.name)}</span>
      <span className="sname">
        {file.name}
        <span className="smeta">· {file.contentType}</span>
      </span>
      <span className="open" aria-hidden="true">
        ↗
      </span>
    </a>
  );
}

// CopyButton copies the answer text and confirms with a brief label swap.
function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      // Clipboard is best-effort; a denied permission just leaves the label unchanged.
    }
  }

  return (
    <button className="copy" type="button" onClick={copy}>
      <span aria-hidden="true">⧉</span> {copied ? "Copied" : "Copy"}
    </button>
  );
}

// Reply renders one grounded answer: the model's prose over the files it drew from, each openable.
// When the model gives no prose (a plain find), it shows how many files matched over the same list.
export function Reply({ outcome }: { outcome: AskOutcome }) {
  const { answer, results } = outcome;
  const found = results.length === 1 ? "Found · 1 file" : `Found · ${results.length} files`;

  return (
    <article className="reply">
      <div className="reply-head">
        <span className="mark" aria-hidden="true">
          ✦
        </span>
        {answer ? <span className="label">Answer</span> : <span className="foundcount">{found}</span>}
        {answer && (
          <>
            <span className="grow" />
            <CopyButton text={answer} />
          </>
        )}
      </div>

      {answer && <p className="prose">{answer}</p>}

      {results.length > 0 &&
        (answer ? (
          <div className="grounded">
            <span className="glabel">Grounded in</span>
            <div className="sources">
              {results.map((result) => (
                <Source key={result.file.id} {...result} />
              ))}
            </div>
          </div>
        ) : (
          <div className="sources">
            {results.map((result) => (
              <Source key={result.file.id} {...result} />
            ))}
          </div>
        ))}
    </article>
  );
}
