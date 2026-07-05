import type { ApiClient } from "../api/client";

// deleteFile removes a file end-to-end: its record, its bytes, and its search vector,
// via DELETE /files/{id}.
export async function deleteFile(api: ApiClient, id: string): Promise<void> {
  const { error } = await api.DELETE("/files/{id}", { params: { path: { id } } });
  if (error) {
    throw new Error("could not delete the file");
  }
}
