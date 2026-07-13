import { describe, expect, it } from "vitest";

import { collectFiles, filterFiles, type FsEntry } from "./collectFiles";

// fileEntry builds a fake file entry yielding a File with the given content (empty means zero-byte).
function fileEntry(name: string, content = "x"): FsEntry {
  return {
    isFile: true,
    isDirectory: false,
    name,
    file: (ok) => ok(new File(content ? [content] : [], name)),
  };
}

// dirEntry builds a fake directory whose reader serves children in the given batches, then empties,
// mirroring the real entries API which returns at most a batch per readEntries call.
function dirEntry(name: string, batches: FsEntry[][]): FsEntry {
  let call = 0;
  return {
    isFile: false,
    isDirectory: true,
    name,
    createReader: () => ({
      readEntries: (ok) => ok(call < batches.length ? batches[call++] : []),
    }),
  };
}

describe("collectFiles", () => {
  it("walks nested directories into a flat list, skipping junk, empties, and system dirs", async () => {
    // Arrange
    const tree: FsEntry[] = [
      dirEntry("album", [
        [
          fileEntry("photo.jpg"),
          fileEntry(".DS_Store"),
          fileEntry("empty", ""),
          dirEntry("sub", [[fileEntry("note.txt")]]),
          dirEntry("__MACOSX", [[fileEntry("._photo.jpg")]]),
        ],
      ]),
    ];

    // Act
    const files = await collectFiles(tree);

    // Assert: only the two real files survive, from any depth.
    expect(files.map((f) => f.name).sort()).toEqual(["note.txt", "photo.jpg"]);
  });

  it("skips a single unreadable entry rather than losing the whole drop", async () => {
    // Arrange: one file entry errors when its File is read.
    const broken: FsEntry = {
      isFile: true,
      isDirectory: false,
      name: "broken",
      file: (_ok, err) => err?.(new Error("read failed")),
    };
    const tree: FsEntry[] = [dirEntry("album", [[fileEntry("photo.jpg"), broken, fileEntry("note.txt")]])];

    // Act
    const files = await collectFiles(tree);

    // Assert: the good files still come through.
    expect(files.map((f) => f.name).sort()).toEqual(["note.txt", "photo.jpg"]);
  });

  it("drains a directory that returns its children across several batches", async () => {
    // Arrange: the reader serves two batches before it empties.
    const tree: FsEntry[] = [dirEntry("big", [[fileEntry("a"), fileEntry("b")], [fileEntry("c")]])];

    // Act
    const files = await collectFiles(tree);

    // Assert
    expect(files.map((f) => f.name).sort()).toEqual(["a", "b", "c"]);
  });
});

describe("filterFiles", () => {
  it("drops system files, forks, __MACOSX paths, and empty files from a flat list", () => {
    // Arrange: files as a folder picker yields them, with webkitRelativePath set.
    const withPath = (name: string, content: string, path: string): File => {
      const file = new File(content ? [content] : [], name);
      Object.defineProperty(file, "webkitRelativePath", { value: path });
      return file;
    };
    const files = [
      withPath("photo.jpg", "x", "album/photo.jpg"),
      withPath(".DS_Store", "x", "album/.DS_Store"),
      withPath("._fork", "x", "album/__MACOSX/._fork"),
      withPath("empty", "", "album/empty"),
    ];

    // Act + Assert
    expect(filterFiles(files).map((f) => f.name)).toEqual(["photo.jpg"]);
  });
});
