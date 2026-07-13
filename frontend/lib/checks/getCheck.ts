import type { ApiClient } from "../api/client";
import type { Check } from "./check";

// getCheck reads one check with whatever claims and verdicts the pipeline has produced so far.
export async function getCheck(api: ApiClient, id: string): Promise<Check> {
  const { data, error } = await api.GET("/checks/{id}", { params: { path: { id } } });
  if (!data) {
    throw new Error(error?.error ?? "could not read the check");
  }
  return data;
}
