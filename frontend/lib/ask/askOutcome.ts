import type { AskResult } from "./askResult";

// AskOutcome is the result of an ask: a human-readable answer (may be empty) and the matched files.
export type AskOutcome = {
  answer: string;
  results: AskResult[];
};
