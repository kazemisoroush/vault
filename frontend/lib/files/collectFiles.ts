// collectFiles walks dropped filesystem entries into a flat list of the real files inside them, so
// a dropped folder ingests exactly like its files dropped one by one. The browser's FileSystem
// entries API is abstracted behind small interfaces, so the walk is testable without a DOM.

// DirEntryReader reads a directory's children, at most a batch per call, until it returns none.
export interface DirEntryReader {
  readEntries(onOk: (entries: FsEntry[]) => void, onErr?: (error: unknown) => void): void;
}

// FsEntry is the slice of FileSystemEntry we use: a file yields its File, a directory yields a reader.
export interface FsEntry {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  file?(onOk: (file: File) => void, onErr?: (error: unknown) => void): void;
  createReader?(): DirEntryReader;
}

// isSystemDir names hold no user files, so we skip them without descending.
function isSystemDir(name: string): boolean {
  return name === "__MACOSX";
}

// keepFile drops archiver bookkeeping (.DS_Store, ._ forks) and empty files, matching the skip
// rules the backend applies when it unpacks an archive.
export function keepFile(file: File): boolean {
  if (file.name === ".DS_Store" || file.name.startsWith("._")) return false;
  return file.size > 0;
}

// entryFile promisifies a file entry's File.
function entryFile(entry: FsEntry): Promise<File> {
  return new Promise((resolve, reject) => {
    if (!entry.file) {
      reject(new Error(`entry ${entry.name} is not a file`));
      return;
    }
    entry.file(resolve, reject);
  });
}

// readAll drains a directory reader, which returns at most a batch per call, until it is empty.
async function readAll(reader: DirEntryReader): Promise<FsEntry[]> {
  const all: FsEntry[] = [];
  for (;;) {
    const batch = await new Promise<FsEntry[]>((resolve, reject) => reader.readEntries(resolve, reject));
    if (batch.length === 0) break;
    all.push(...batch);
  }
  return all;
}

// collectFiles walks the entries depth-first into a flat, filtered list of files.
export async function collectFiles(entries: FsEntry[]): Promise<File[]> {
  const files: File[] = [];
  for (const entry of entries) {
    if (entry.isDirectory) {
      if (isSystemDir(entry.name) || !entry.createReader) continue;
      const children = await readAll(entry.createReader());
      files.push(...(await collectFiles(children)));
    } else if (entry.isFile) {
      const file = await entryFile(entry);
      if (keepFile(file)) files.push(file);
    }
  }
  return files;
}

// filterFiles applies the same skip rules to a flat FileList, for the picker paths: the multi-file
// picker and the folder picker, whose files carry a webkitRelativePath naming their folder tree.
export function filterFiles(files: File[]): File[] {
  return files.filter((file) => {
    const path = (file as File & { webkitRelativePath?: string }).webkitRelativePath ?? "";
    if (path.split("/").some((segment) => segment === "__MACOSX")) return false;
    return keepFile(file);
  });
}
