import type { ApiClient } from "../api/client";
import type { AskResult } from "./askResult";

// ask sends a natural-language query to the retrieval endpoint and returns the matches.
export async function ask(api: ApiClient, query: string): Promise<AskResult[]> {
  const { data, error } = await api.POST("/ask", { body: { query } });
  if (error || !data) {
    throw new Error("search failed");
  }
  return data.results;
}
