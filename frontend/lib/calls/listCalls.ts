import type { ApiClient } from "../api/client";
import type { LlmCall } from "./llmCall";

// listCalls returns the recent LLM calls, newest first, for the trace view.
export async function listCalls(api: ApiClient): Promise<LlmCall[]> {
  const { data } = await api.GET("/calls", {});
  return data?.calls ?? [];
}
