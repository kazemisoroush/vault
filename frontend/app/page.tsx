"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

import { Answer } from "../components/Answer";
import { AskBox } from "../components/AskBox";
import { DropZone } from "../components/DropZone";
import { FileList } from "../components/FileList";
import { Results } from "../components/Results";
import { ThemeToggle } from "../components/ThemeToggle";
import { Trace } from "../components/Trace";
import { Wordmark } from "../components/Wordmark";
import { ask } from "../lib/ask/ask";
import type { AskOutcome } from "../lib/ask/askOutcome";
import { useAuth } from "../lib/auth/context";
import { listCalls } from "../lib/calls/listCalls";
import type { LlmCall } from "../lib/calls/llmCall";
import { deleteFile } from "../lib/files/deleteFile";
import { dropFile } from "../lib/files/dropFile";
import { StreamingSha256 } from "../lib/files/streamingSha256";
import { listFiles } from "../lib/files/listFiles";
import type { VaultFile } from "../lib/files/vaultFile";

const pollInterval = 3000;

export default function Home() {
  const { ready, authenticated, api, signOut } = useAuth();
  const router = useRouter();
  const [files, setFiles] = useState<VaultFile[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [outcome, setOutcome] = useState<AskOutcome | null>(null);
  const [asking, setAsking] = useState(false);
  const [calls, setCalls] = useState<LlmCall[]>([]);

  useEffect(() => {
    if (ready && !authenticated) {
      router.replace("/login");
    }
  }, [ready, authenticated, router]);

  const refresh = useCallback(async () => {
    if (!api) return;
    setFiles(await listFiles(api));
  }, [api]);

  const refreshCalls = useCallback(async () => {
    if (!api) return;
    setCalls(await listCalls(api));
  }, [api]);

  useEffect(() => {
    if (authenticated) {
      void refresh();
      void refreshCalls();
    }
  }, [authenticated, refresh, refreshCalls]);

  // Keep refreshing files and the call trace so async extraction shows up as it happens.
  useEffect(() => {
    if (!authenticated) return;
    const timer = setInterval(() => {
      void refresh();
      void refreshCalls();
    }, pollInterval);
    return () => clearInterval(timer);
  }, [authenticated, refresh, refreshCalls]);

  const hasher = useMemo(() => new StreamingSha256(), []);

  const onFile = useCallback(
    async (file: File) => {
      if (!api) return;
      setBusy(true);
      setError(null);
      try {
        await dropFile(api, file, hasher);
        await refresh();
      } catch (err) {
        setError(err instanceof Error ? err.message : "drop failed");
      } finally {
        setBusy(false);
      }
    },
    [api, hasher, refresh],
  );

  const onDelete = useCallback(
    async (id: string) => {
      if (!api) return;
      setError(null);
      try {
        await deleteFile(api, id);
        await refresh();
      } catch (err) {
        setError(err instanceof Error ? err.message : "delete failed");
      }
    },
    [api, refresh],
  );

  const onAsk = useCallback(
    async (query: string) => {
      if (!api) return;
      setAsking(true);
      setError(null);
      try {
        setOutcome(await ask(api, query));
        await refreshCalls();
      } catch (err) {
        setError(err instanceof Error ? err.message : "search failed");
      } finally {
        setAsking(false);
      }
    },
    [api, refreshCalls],
  );

  if (!ready) {
    return (
      <main className="loading">
        <p>Loading…</p>
      </main>
    );
  }
  if (!authenticated) {
    return null;
  }

  return (
    <main className="shell">
      <header className="topbar">
        <Wordmark />
        <span className="spacer" />
        <ThemeToggle />
        <button className="ghost" onClick={signOut}>
          Sign out
        </button>
      </header>

      <h1 className="greeting">Your vault</h1>
      <p className="sub">Ask for anything, or drop a file to keep it.</p>

      <AskBox onAsk={onAsk} busy={asking} />
      {outcome !== null && (
        <>
          {/* The answer's source is the first result: the model puts it first when it answers. */}
          {outcome.answer && <Answer answer={outcome.answer} source={outcome.results[0]?.file.name} />}
          <p className="eyebrow">{outcome.results.length === 1 ? "1 result" : `${outcome.results.length} results`}</p>
          <Results results={outcome.results} />
        </>
      )}
      {error && <p role="alert">{error}</p>}

      <p className="eyebrow">Keep something new</p>
      <div className="panel">
        <DropZone onFile={onFile} busy={busy} />
        <FileList files={files} onDelete={onDelete} />
      </div>

      <Trace calls={calls} />
    </main>
  );
}
