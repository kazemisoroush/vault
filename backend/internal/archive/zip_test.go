package archive

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zipEntry is one file (a directory when body is empty and the name ends in a slash) for a test zip.
type zipEntry struct {
	name string
	body string
}

// makeZip builds an in-memory zip from the given entries, preserving their order.
func makeZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.name)
		require.NoError(t, err)
		if e.body != "" {
			_, err = f.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestZipUnpackReturnsOnlyRealFiles(t *testing.T) {
	// Arrange: two real files plus a directory, macOS junk, and a nested archive.
	content := makeZip(t, []zipEntry{
		{name: "photo.jpg", body: "image bytes"},
		{name: "docs/", body: ""},
		{name: "docs/note.txt", body: "hello there"},
		{name: "__MACOSX/photo.jpg", body: "resource fork"},
		{name: ".DS_Store", body: "finder junk"},
		{name: "inner.zip", body: "PK\x03\x04nested"},
	})

	// Act
	files, err := Zip{}.Unpack(content)

	// Assert: only the two real files come back, with their full paths and guessed types.
	require.NoError(t, err)
	require.Len(t, files, 2)
	byName := map[string]File{}
	for _, f := range files {
		byName[f.Name] = f
	}
	require.Contains(t, byName, "photo.jpg")
	require.Contains(t, byName, "docs/note.txt")
	assert.Equal(t, "image/jpeg", byName["photo.jpg"].ContentType)
	assert.True(t, strings.HasPrefix(byName["docs/note.txt"].ContentType, "text/plain"))
	assert.Equal(t, "hello there", string(byName["docs/note.txt"].Data))
}

func TestZipUnpackReturnsEmptyForAJunkOnlyArchive(t *testing.T) {
	// Arrange: nothing but a directory and macOS bookkeeping.
	content := makeZip(t, []zipEntry{
		{name: "folder/", body: ""},
		{name: "__MACOSX/folder/file", body: "fork"},
	})

	// Act
	files, err := Zip{}.Unpack(content)

	// Assert: no real files, so the caller can treat the archive as empty.
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestZipUnpackRejectsACorruptArchive(t *testing.T) {
	// Act: zip magic but not a valid archive.
	_, err := Zip{}.Unpack([]byte("PK\x03\x04 not really a zip"))

	// Assert
	require.Error(t, err)
}

func TestIsZipNeedsBothTypeAndMagic(t *testing.T) {
	zipped := makeZip(t, []zipEntry{{name: "a.txt", body: "x"}})
	assert.True(t, IsZip(zipped, "application/zip"))
	// A docx is a zip on disk but arrives with its own type, so it is left whole.
	assert.False(t, IsZip(zipped, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
	// A zip content type without the magic bytes is not torn apart.
	assert.False(t, IsZip([]byte("plain text"), "application/zip"))
}
