import { sha256 } from "js-sha256";

import type { ContentHasher } from "./contentHasher";

// chunkSize bounds how much of the file is held in memory at once while hashing.
const chunkSize = 8 * 1024 * 1024;

// StreamingSha256 hashes a file as SHA-256 hex, reading it in chunks so any size stays low-memory.
export class StreamingSha256 implements ContentHasher {
  async hash(file: File): Promise<string> {
    const hasher = sha256.create();
    for (let offset = 0; offset < file.size; offset += chunkSize) {
      const buffer = await file.slice(offset, offset + chunkSize).arrayBuffer();
      hasher.update(buffer);
    }
    return hasher.hex();
  }
}
