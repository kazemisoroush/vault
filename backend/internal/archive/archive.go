// Package archive unpacks a container file, such as a zip, into the individual files inside it. It
// holds no storage or pipeline knowledge: it turns bytes into a list of inner files, and the caller
// decides what to do with them.
package archive

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

// Unpacker turns an archive's bytes into the files inside it.
type Unpacker interface {
	Unpack(content []byte) ([]File, error)
}
