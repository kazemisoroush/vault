import type { ApiClient } from "../api/client";
import type { AskOutcome } from "./askOutcome";

// ask sends a natural-language query and returns the answer and the matched files.
export async function ask(api: ApiClient, query: string): Promise<AskOutcome> {
  const { data, error } = await api.POST("/ask", { body: { query } });
  if (error || !data) {
    throw new Error("search failed");
  }
  return { answer: data.answer ?? "", results: data.results };
}
