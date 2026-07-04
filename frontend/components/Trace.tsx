"use client";

import { Fragment, useState } from "react";

import type { LlmCall } from "../lib/calls/llmCall";

// Trace shows recent LLM calls; a row expands to reveal the prompt and the raw reply.
export function Trace({ calls }: { calls: LlmCall[] }) {
  const [open, setOpen] = useState<string | null>(null);

  if (calls.length === 0) {
    return null;
  }

  return (
    <section className="trace">
      <h2>Recent LLM calls</h2>
      <table>
        <thead>
          <tr>
            <th>op</th>
            <th>latency</th>
            <th>tokens in/out</th>
            <th>status</th>
            <th>when</th>
          </tr>
        </thead>
        <tbody>
          {calls.map((call, index) => {
            const key = `${call.createdAt}-${index}`;
            return (
              <Fragment key={key}>
                <tr className="row" onClick={() => setOpen(open === key ? null : key)}>
                  <td>{call.op}</td>
                  <td>{call.latencyMs} ms</td>
                  <td>
                    {call.inputTokens}/{call.outputTokens}
                  </td>
                  <td className={call.ok ? "ok" : "fail"}>{call.ok ? "ok" : "failed"}</td>
                  <td>{new Date(call.createdAt).toLocaleTimeString()}</td>
                </tr>
                {open === key && (
                  <tr className="detail">
                    <td colSpan={5}>
                      <p className="muted">prompt</p>
                      <pre>{call.prompt}</pre>
                      <p className="muted">reply</p>
                      <pre>{call.error ? call.error : call.reply}</pre>
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </section>
  );
}
