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
      <div className="tracewrap">
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
                    <td>
                      <span className="op">{call.op}</span>
                    </td>
                    <td className="num">{call.latencyMs} ms</td>
                    <td className="num">
                      {call.inputTokens} / {call.outputTokens}
                    </td>
                    <td>
                      <span className={call.ok ? "dot ok" : "dot fail"}>{call.ok ? "ok" : "failed"}</span>
                    </td>
                    <td className="when">{new Date(call.createdAt).toLocaleTimeString()}</td>
                  </tr>
                  {open === key && (
                    <tr className="detail">
                      <td colSpan={5}>
                        <div className="kv">
                          <span className="k">prompt</span>
                          <pre>{call.prompt}</pre>
                        </div>
                        <div className="kv">
                          <span className="k">reply</span>
                          <pre>{call.error ? call.error : call.reply}</pre>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
