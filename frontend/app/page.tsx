"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { AskBox } from "../components/AskBox";
import { DropZone } from "../components/DropZone";
import { FileList } from "../components/FileList";
import { Results } from "../components/Results";
import { Trace } from "../components/Trace";
import { ask } from "../lib/ask/ask";
import type { AskResult } from "../lib/ask/askResult";
import { useAuth } from "../lib/auth/context";
import { listCalls } from "../lib/calls/listCalls";
import type { LlmCall } from "../lib/calls/llmCall";
import { dropFile } from "../lib/files/dropFile";
import { listFiles } from "../lib/files/listFiles";
import type { VaultFile } from "../lib/files/vaultFile";

const pollInterval = 3000;

export default function Home() {
  const { ready, authenticated, api, signOut } = useAuth();
  const router = useRouter();
  const [files, setFiles] = useState<VaultFile[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [results, setResults] = useState<AskResult[] | null>(null);
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

  const onFile = useCallback(
    async (file: File) => {
      if (!api) return;
      setBusy(true);
      setError(null);
      try {
        await dropFile(api, file);
        await refresh();
      } catch (err) {
        setError(err instanceof Error ? err.message : "drop failed");
      } finally {
        setBusy(false);
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
        setResults(await ask(api, query));
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
      <main>
        <p>Loading…</p>
      </main>
    );
  }
  if (!authenticated) {
    return null;
  }

  return (
    <main className="home">
      <header className="bar">
        <h1>Your vault</h1>
        <button className="link" onClick={signOut}>
          Sign out
        </button>
      </header>
      <AskBox onAsk={onAsk} busy={asking} />
      {results !== null && <Results results={results} />}
      {error && <p role="alert">{error}</p>}
      <section className="drop">
        <DropZone onFile={onFile} busy={busy} />
        <FileList files={files} />
      </section>
      <Trace calls={calls} />
    </main>
  );
}
