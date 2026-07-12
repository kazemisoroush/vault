package ingest_test

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
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

func TestZipExpandsEachInnerFileAsAStagedUpload(t *testing.T) {
	// Arrange: an archive with two real files, plus a directory, macOS junk, and a nested zip.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	content := makeZip(t, []zipEntry{
		{name: "photo.jpg", body: "the image bytes"},
		{name: "docs/", body: ""},
		{name: "docs/note.txt", body: "hello there"},
		{name: "__MACOSX/photo.jpg", body: "resource fork"},
		{name: ".DS_Store", body: "finder junk"},
		{name: "inner.zip", body: "PK\x03\x04nested"},
	})
	staging := "uploads/upl-1"
	pending := domain.File{ID: "upl-1", OwnerID: "alice", Key: staging, Name: "album.zip", ContentType: "application/zip", Status: domain.StatusPending}

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "application/zip", nil)

	// Only the two real files are staged: a pending record then its object, each.
	var children []domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, f domain.File) error {
		children = append(children, f)
		return nil
	})
	blobs.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
	// The archive itself is discarded, not stored: its record and staging object are removed.
	idx.EXPECT().Delete(gomock.Any(), "upl-1").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.NewHandler(idx, blobs, mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: only photo.jpg and note.txt were staged, owned by the archive owner, typed by name.
	require.NoError(t, err)
	require.Len(t, children, 2)
	names := map[string]string{}
	for _, c := range children {
		names[c.Name] = c.ContentType
		assert.Equal(t, "alice", c.OwnerID)
		assert.Equal(t, domain.StatusPending, c.Status)
		assert.True(t, strings.HasPrefix(c.Key, "uploads/"))
	}
	assert.Contains(t, names, "photo.jpg")
	assert.Contains(t, names, "note.txt")
	assert.Equal(t, "image/jpeg", names["photo.jpg"])
	assert.True(t, strings.HasPrefix(names["note.txt"], "text/plain"))
}

func TestZipOverSizeCapMarksFailedWithoutLoading(t *testing.T) {
	// Arrange: the S3 event reports a size over the cap, so the object must never be read.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	staging := "uploads/upl-1"
	pending := domain.File{ID: "upl-1", Key: staging, Name: "huge.zip", ContentType: "application/zip", Status: domain.StatusPending}
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)
	// No blobs.Get: an over-cap archive is rejected before it is loaded.

	h := ingest.NewHandler(idx, blobs, mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act: the event's object size, not the client-declared record size, drives the guard.
	err := h.Handle(context.Background(), s3EventSized(staging, (512<<20)+1))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, saved.Status)
	assert.Equal(t, "upl-1", saved.ID)
}

func TestZipWithNoRealFilesMarksFailed(t *testing.T) {
	// Arrange: an archive holding only a directory and macOS bookkeeping.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	content := makeZip(t, []zipEntry{
		{name: "folder/", body: ""},
		{name: "__MACOSX/folder/x", body: "fork"},
	})
	staging := "uploads/upl-1"
	pending := domain.File{ID: "upl-1", Key: staging, Name: "empty.zip", ContentType: "application/zip", Status: domain.StatusPending}
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "application/zip", nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.NewHandler(idx, blobs, mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: no children staged, the archive is failed rather than vanishing.
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, saved.Status)
}

func TestCorruptZipMarksFailed(t *testing.T) {
	// Arrange: zip magic but a body that is not a valid archive.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	staging := "uploads/upl-1"
	pending := domain.File{ID: "upl-1", Key: staging, Name: "broken.zip", ContentType: "application/zip", Status: domain.StatusPending}
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return([]byte("PK\x03\x04 not really a zip"), "application/zip", nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.NewHandler(idx, blobs, mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, saved.Status)
}

func TestSettledArchiveRedeliveryIsNoOp(t *testing.T) {
	// Arrange: a redelivered event for an archive already settled to failed (its staging is gone).
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", ContentType: "application/zip", Status: domain.StatusFailed}, nil)

	h := ingest.NewHandler(idx, mocks.NewMockStore(ctrl), mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event("uploads/upl-1"))

	// Assert: no read, no re-processing; the missing staging object is never touched.
	require.NoError(t, err)
}
