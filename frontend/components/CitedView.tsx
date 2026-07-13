"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import type { ApiClient } from "../lib/api/client";
import type { Check } from "../lib/checks/check";
import { createCheck } from "../lib/checks/createCheck";
import { getCheck } from "../lib/checks/getCheck";
import type { VaultFile } from "../lib/files/vaultFile";
import { DraftPanel } from "./DraftPanel";
import { RecordPanel } from "./RecordPanel";

const defaultPollMs = 3000;

// CitedView is the legal face: the record (your files) on the left, the draft being checked on
// the right. It owns the check lifecycle: submit, poll while the pipeline runs, and hold the
// selected claim whose references the left panel shows. pollMs is overridable so tests can
// poll fast with real timers.
export function CitedView({
  api,
  files,
  pollMs = defaultPollMs,
}: {
  api: ApiClient;
  files: VaultFile[];
  pollMs?: number;
}) {
  const [check, setCheck] = useState<Check | null>(null);
  const [selected, setSelected] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const checkID = useRef<string | null>(null);

  const onCheck = useCallback(
    async (text: string) => {
      setSubmitting(true);
      setError(null);
      setSelected(null);
      try {
        const created = await createCheck(api, text);
        checkID.current = created.id;
        setCheck(created);
      } catch (err) {
        setError(err instanceof Error ? err.message : "check failed to start");
      } finally {
        setSubmitting(false);
      }
    },
    [api],
  );

  // Poll the running check until the pipeline lands on done or failed.
  useEffect(() => {
    if (!check || (check.status !== "pending" && check.status !== "running")) return;
    const timer = setInterval(() => {
      const id = checkID.current;
      if (!id) return;
      getCheck(api, id)
        .then(setCheck)
        .catch((err: unknown) => {
          setError(err instanceof Error ? err.message : "could not read the check");
        });
    }, pollMs);
    return () => clearInterval(timer);
  }, [api, check, pollMs]);

  const claim = check && selected !== null ? (check.claims ?? [])[selected] : undefined;

  return (
    <div className="cited">
      <RecordPanel files={files} claim={claim} onBack={() => setSelected(null)} />
      <DraftPanel
        check={check}
        submitting={submitting}
        selected={selected}
        onCheck={onCheck}
        onSelect={setSelected}
      />
      {error && (
        <p role="alert" className="cited-error">
          {error}
        </p>
      )}
    </div>
  );
}
