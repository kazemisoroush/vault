// Package archive unpacks a container file, such as a zip, into the individual files inside it. It
// holds no storage or pipeline knowledge: it turns bytes into a stream of inner files, and the
// caller decides what to do with each.
package archive

import "iter"

// File is one file pulled out of an archive.
type File struct {
	// Name is the entry's path within the archive, unique per archive. The caller may base-name it
	// for display but should key on the full path.
	Name string
	// ContentType is guessed from the entry name, so the file is typed like a normal upload.
	ContentType string
	// Data is the entry's bytes.
	Data []byte
}

// Unpacker reads an archive and yields the files inside it one at a time, so a caller never holds
// the whole decompressed archive in memory at once. A failure to open the archive is yielded as a
// zero File with a non-nil error; a per-entry read failure is skipped rather than yielded.
type Unpacker interface {
	Unpack(content []byte) iter.Seq2[File, error]
}
