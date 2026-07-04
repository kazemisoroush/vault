import type { ApiClient } from "../api/client";
import type { VaultFile } from "./vaultFile";

// listFiles returns the current page of file records.
export async function listFiles(api: ApiClient): Promise<VaultFile[]> {
  const { data } = await api.GET("/files", {});
  return data?.files ?? [];
}
