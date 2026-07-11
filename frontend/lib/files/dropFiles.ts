import type { ApiClient } from "../api/client";
import { dropFile } from "./dropFile";
import type { VaultFile } from "./vaultFile";

// uploadConcurrency is how many files the queue uploads at once: enough to feel fast, gentle
// enough not to open a flood of presigned PUTs at once.
const uploadConcurrency = 3;

// DropFailure is one file that did not upload, with the reason.
export interface DropFailure {
  file: File;
  error: Error;
}

// DropProgress reports how many of the batch have finished so the UI can show a count.
export interface DropProgress {
  done: number;
  total: number;
}

// DropResult is the outcome of a batch: the files that landed and the ones that did not.
export interface DropResult {
  uploaded: VaultFile[];
  failed: DropFailure[];
}

// dropFiles uploads every file through dropFile, a few at a time, calling onProgress as each one
// finishes. A failed file is collected rather than aborting the batch, so the rest still land.
export async function dropFiles(
  api: ApiClient,
  files: File[],
  onProgress?: (progress: DropProgress) => void,
): Promise<DropResult> {
  const total = files.length;
  const uploaded: VaultFile[] = [];
  const failed: DropFailure[] = [];
  let done = 0;
  let next = 0;

  async function worker(): Promise<void> {
    while (next < files.length) {
      const file = files[next];
      next += 1;
      try {
        uploaded.push(await dropFile(api, file));
      } catch (err) {
        failed.push({ file, error: err instanceof Error ? err : new Error("drop failed") });
      } finally {
        done += 1;
        onProgress?.({ done, total });
      }
    }
  }

  const workerCount = Math.min(uploadConcurrency, files.length);
  await Promise.all(Array.from({ length: workerCount }, () => worker()));
  return { uploaded, failed };
}
