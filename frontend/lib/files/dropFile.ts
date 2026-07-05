import type { ApiClient } from "../api/client";
import type { ContentHasher } from "./contentHasher";
import { uploadBytes } from "./upload";
import type { VaultFile } from "./vaultFile";

// dropFile hashes the content, registers it (an identical file is a no-op), then uploads the bytes
// only when the file is new.
export async function dropFile(api: ApiClient, file: File, hasher: ContentHasher): Promise<VaultFile> {
  const contentType = file.type || "application/octet-stream";
  const hash = await hasher.hash(file);

  const { data, error } = await api.POST("/files", {
    body: { name: file.name, contentType, size: file.size, hash },
  });
  if (error || !data) {
    throw new Error("could not register the file");
  }

  if (data.uploadUrl) {
    await uploadBytes(data.uploadUrl, file, contentType);
  }
  return data.file;
}
