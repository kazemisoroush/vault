"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import type { ApiClient } from "../lib/api/client";
import type { Check } from "../lib/checks/check";
import { createCheck } from "../lib/checks/createCheck";
import { getCheck } from "../lib/checks/getCheck";
import { CheckBox } from "./CheckBox";
import { CheckResult } from "./CheckResult";

const defaultPollMs = 3000;

// CheckPanel owns the check lifecycle: submit, poll until landed, render the result.
export function CheckPanel({ api, pollMs = defaultPollMs }: { api: ApiClient; pollMs?: number }) {
  const [check, setCheck] = useState<Check | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const checkID = useRef<string | null>(null);

  const onReset = useCallback(() => {
    checkID.current = null;
    setCheck(null);
    setError(null);
  }, []);

  const onCheck = useCallback(
    async (text: string) => {
      setSubmitting(true);
      setError(null);
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
        .then((fetched) => {
          // A response for a reset or superseded check is ignored, errors included.
          if (checkID.current !== id) return;
          setError(null);
          setCheck((previous) => {
            // A stale or out-of-order response must never revert a landed check.
            if (previous && (previous.status === "done" || previous.status === "failed")) {
              return previous;
            }
            return fetched;
          });
          if (fetched.status === "done" || fetched.status === "failed") {
            checkID.current = null;
          }
        })
        .catch((err: unknown) => {
          if (checkID.current !== id) return;
          setError(err instanceof Error ? err.message : "could not read the check");
        });
    }, pollMs);
    return () => clearInterval(timer);
  }, [api, check, pollMs]);

  return (
    <>
      {check === null && <CheckBox onCheck={onCheck} busy={submitting} />}
      {check !== null && (check.status === "pending" || check.status === "running") && (
        <p className="check-status">Checking… every sentence is being matched against your documents.</p>
      )}
      {check !== null && check.status === "failed" && (
        <div className="panel check-result">
          <p role="alert">This check failed to finish. Try a shorter text, or try again.</p>
          <button className="btn" type="button" onClick={onReset}>
            Check another
          </button>
        </div>
      )}
      {check !== null && check.status === "done" && <CheckResult check={check} onReset={onReset} />}
      {error && <p role="alert">{error}</p>}
    </>
  );
}
