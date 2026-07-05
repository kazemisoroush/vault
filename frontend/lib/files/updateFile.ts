import type { ApiClient } from "../api/client";

// updateFile renames a file by id via PATCH /files/{id}.
export async function updateFile(api: ApiClient, id: string, name: string): Promise<void> {
  const { error } = await api.PATCH("/files/{id}", { params: { path: { id } }, body: { name } });
  if (error) {
    throw new Error("could not update the file");
  }
}
