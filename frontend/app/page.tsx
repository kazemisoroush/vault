"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { AskBox } from "../components/AskBox";
import { CitedView } from "../components/CitedView";
import { DropZone } from "../components/DropZone";
import { FileList } from "../components/FileList";
import { ModeToggle } from "../components/ModeToggle";
import { Reply } from "../components/Reply";
import { ThemeToggle } from "../components/ThemeToggle";
import { Trace } from "../components/Trace";
import { Wordmark } from "../components/Wordmark";
import { ask } from "../lib/ask/ask";
import type { AskOutcome } from "../lib/ask/askOutcome";
import { useAuth } from "../lib/auth/context";
import type { Mode } from "../lib/mode";
import { listCalls } from "../lib/calls/listCalls";
import type { LlmCall } from "../lib/calls/llmCall";
import { deleteFile } from "../lib/files/deleteFile";
import { dropFiles } from "../lib/files/dropFiles";
import { listFiles } from "../lib/files/listFiles";
import type { VaultFile } from "../lib/files/vaultFile";
import { reportTimeToFile } from "../lib/metrics/reportTimeToFile";

const pollInterval = 3000;

export default function Home() {
  const { ready, authenticated, api, signOut } = useAuth();
  const router = useRouter();
  const [files, setFiles] = useState<VaultFile[]>([]);
  const [pending, setPending] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [outcome, setOutcome] = useState<AskOutcome | null>(null);
  const [asking, setAsking] = useState(false);
  const [calls, setCalls] = useState<LlmCall[]>([]);
  const [mode, setMode] = useState<Mode>("personal");

  // The pre-paint script in the layout stamps the persisted mode; the state follows it here.
  useEffect(() => {
    if (document.documentElement.dataset.mode === "legal") {
      setMode("legal");
    }
  }, []);

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

  const onFiles = useCallback(
    async (chosen: File[]) => {
      if (!api) return;
      setError(null);
      // Track files still in flight so the drop zone can count down as the queue drains.
      setPending((count) => count + chosen.length);
      try {
        const { failed } = await dropFiles(api, chosen, () =>
          setPending((count) => Math.max(0, count - 1)),
        );
        await refresh();
        if (failed.length > 0) {
          const names = failed.map((f) => f.file.name).join(", ");
          setError(failed.length === 1 ? `${names} did not upload` : `${failed.length} files did not upload: ${names}`);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "drop failed");
        setPending(0);
      }
    },
    [api, refresh],
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
        // Time-to-file: from the phrase submitted to the results shown, reported as the one metric.
        const start = performance.now();
        setOutcome(await ask(api, query));
        void reportTimeToFile(api, performance.now() - start);
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
        <Wordmark mode={mode} />
        <span className="spacer" />
        <ModeToggle mode={mode} onMode={setMode} />
        <ThemeToggle />
        <button className="ghost" onClick={signOut}>
          Sign out
        </button>
      </header>

      {mode === "legal" ? (
        api && (
          <>
            <h1 className="greeting">Checked against your record</h1>
            <p className="sub">Paste any text. Every sentence answers to your documents.</p>
            <CitedView api={api} files={files} />
          </>
        )
      ) : (
        <>
          <h1 className="greeting">Your vault</h1>
          <p className="sub">Ask for anything, or drop a file to keep it.</p>

          <AskBox onAsk={onAsk} busy={asking} />
          {outcome !== null && <Reply outcome={outcome} />}
          {error && <p role="alert">{error}</p>}

          <p className="eyebrow">Keep something new</p>
          <div className="panel">
            <DropZone onFiles={onFiles} busy={pending > 0} pending={pending} />
            <FileList files={files} onDelete={onDelete} />
          </div>

          <Trace calls={calls} />
        </>
      )}
    </main>
  );
}
