"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { DropZone } from "../components/DropZone";
import { FileList } from "../components/FileList";
import { useAuth } from "../lib/auth/context";
import { dropFile } from "../lib/files/dropFile";
import type { VaultFile } from "../lib/files/vaultFile";

const pollInterval = 3000;

export default function Home() {
  const { ready, authenticated, api, signOut } = useAuth();
  const router = useRouter();
  const [files, setFiles] = useState<VaultFile[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (ready && !authenticated) {
      router.replace("/login");
    }
  }, [ready, authenticated, router]);

  const refresh = useCallback(async () => {
    if (!api) return;
    const { data } = await api.GET("/files", {});
    setFiles(data?.files ?? []);
  }, [api]);

  useEffect(() => {
    if (authenticated) void refresh();
  }, [authenticated, refresh]);

  // Keep refreshing while a dropped file is still being extracted.
  useEffect(() => {
    if (!authenticated || !files.some((file) => file.status === "pending")) return;
    const timer = setInterval(() => void refresh(), pollInterval);
    return () => clearInterval(timer);
  }, [authenticated, files, refresh]);

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
      <DropZone onFile={onFile} busy={busy} />
      {error && <p role="alert">{error}</p>}
      <FileList files={files} />
    </main>
  );
}
