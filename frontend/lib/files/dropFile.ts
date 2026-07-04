import type { ApiClient } from "../api/client";
import { uploadBytes } from "./upload";
import type { VaultFile } from "./vaultFile";

// dropFile registers a file record, then PUTs its bytes to the returned presigned URL.
// The extractor fills the metadata once the bytes land, so no metadata is typed here.
export async function dropFile(api: ApiClient, file: File): Promise<VaultFile> {
  const contentType = file.type || "application/octet-stream";

  const { data, error } = await api.POST("/files", {
    body: { name: file.name, contentType, size: file.size },
  });
  if (error || !data) {
    throw new Error("could not register the file");
  }

  await uploadBytes(data.uploadUrl, file, contentType);
  return data.file;
}
