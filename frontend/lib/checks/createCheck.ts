import type { ApiClient } from "../api/client";
import type { Check } from "./check";

// createCheck submits text for verification and returns the pending check to poll.
export async function createCheck(api: ApiClient, text: string): Promise<Check> {
  const { data, error } = await api.POST("/checks", { body: { text } });
  if (!data) {
    throw new Error(error?.error ?? "could not start the check");
  }
  return data;
}
