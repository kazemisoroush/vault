// ContentHasher turns a file's bytes into the content id that identifies it. Swappable so the
// hashing scheme (streaming SHA-256 today) can change without touching the drop flow.
export interface ContentHasher {
  hash(file: File): Promise<string>;
}
