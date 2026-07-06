import type { ApiClient } from "../api/client";

// reportTimeToFile sends the client-measured milliseconds from asking to opening a file, best effort.
export async function reportTimeToFile(api: ApiClient, ms: number): Promise<void> {
  try {
    await api.POST("/metrics/time-to-file", { body: { ms } });
  } catch {
    // Telemetry must never disrupt the user, so a failure here is ignored.
  }
}
