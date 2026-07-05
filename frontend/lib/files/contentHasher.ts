// ContentHasher turns a file's bytes into the content id that identifies it, kept swappable so the
// hashing scheme can change without touching the drop flow.
export interface ContentHasher {
  hash(file: File): Promise<string>;
}
