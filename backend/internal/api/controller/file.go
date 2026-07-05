package controller

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// contentHashPattern matches a SHA-256 hex string, the shape of a content id and its S3 key.
var contentHashPattern = regexp.MustCompile("^[0-9a-f]{64}$")

// FileController serves the five CRUD verbs over file records and their blobs.
type FileController struct {
	index   index.Index
	blobs   blob.Store
	vectors vectors.Store
	now     func() time.Time
}

// NewFileController builds a file controller with a real clock.
func NewFileController(idx index.Index, blobs blob.Store, store vectors.Store) *FileController {
	return &FileController{index: idx, blobs: blobs, vectors: store, now: time.Now}
}

// dropRequest is a POST /files body, identified by its content Hash.
type dropRequest struct {
	Name        string            `json:"name"`
	ContentType string            `json:"contentType"`
	Size        int64             `json:"size"`
	Hash        string            `json:"hash"`
	Meta        map[string]string `json:"meta"`
}

// dropResponse is a POST /files body, with no UploadURL for a duplicate.
type dropResponse struct {
	File      domain.File `json:"file"`
	UploadURL string      `json:"uploadUrl,omitempty"`
}

// Drop registers a file record and returns a presigned upload URL.
func (c *FileController) Drop(w http.ResponseWriter, r *http.Request) {
	var req dropRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.ContentType == "" || req.Hash == "" {
		writeError(w, http.StatusBadRequest, "name, contentType and hash are required")
		return
	}
	if !contentHashPattern.MatchString(req.Hash) {
		writeError(w, http.StatusBadRequest, "hash must be a SHA-256 hex string")
		return
	}

	// The content hash is the file id. If the same bytes are already stored (a ready record) this is
	// a duplicate: return it with no upload URL. A record that never finished uploading (pending or
	// failed) falls through so the drop can be retried.
	existing, err := c.index.Get(r.Context(), req.Hash)
	if err != nil && !errors.Is(err, index.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "could not check the vault")
		return
	}
	if err == nil && existing.Status == domain.StatusReady {
		writeJSON(w, http.StatusOK, dropResponse{File: existing})
		return
	}

	now := c.now().UTC()
	file := domain.File{
		ID:          req.Hash,
		Name:        req.Name,
		ContentType: req.ContentType,
		Size:        req.Size,
		Status:      domain.StatusPending,
		Meta:        req.Meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	file.Key = blob.Key(file.ID)

	uploadURL, err := c.blobs.PresignPut(r.Context(), file.Key, file.ContentType, presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not presign upload")
		return
	}

	if err := c.index.Put(r.Context(), file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save file record")
		return
	}

	writeJSON(w, http.StatusCreated, dropResponse{File: file, UploadURL: uploadURL})
}

// getResponse is the body returned by a GET /files/{id} call.
type getResponse struct {
	File        domain.File `json:"file"`
	DownloadURL string      `json:"downloadUrl"`
}

// Get returns one file record and a presigned download URL.
func (c *FileController) Get(w http.ResponseWriter, r *http.Request) {
	file, ok := lookup(w, r, c.index)
	if !ok {
		return
	}

	downloadURL, err := c.blobs.PresignGet(r.Context(), file.Key, presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not presign download")
		return
	}

	writeJSON(w, http.StatusOK, getResponse{File: file, DownloadURL: downloadURL})
}

// listResponse is the body returned by a GET /files call.
type listResponse struct {
	Files  []domain.File `json:"files"`
	Cursor string        `json:"cursor,omitempty"`
}

// List returns one page of file records.
func (c *FileController) List(w http.ResponseWriter, r *http.Request) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(int32(parsed), maxLimit)
	}

	files, cursor, err := c.index.List(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list files")
		return
	}

	writeJSON(w, http.StatusOK, listResponse{Files: files, Cursor: cursor})
}

// updateRequest is the body of a PATCH /files/{id} call.
type updateRequest struct {
	Name *string            `json:"name"`
	Meta *map[string]string `json:"meta"`
}

// Update changes a file's name or free-form metadata.
func (c *FileController) Update(w http.ResponseWriter, r *http.Request) {
	file, ok := lookup(w, r, c.index)
	if !ok {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == nil && req.Meta == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Name != nil {
		if *req.Name == "" {
			writeError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		file.Name = *req.Name
	}
	if req.Meta != nil {
		file.Meta = *req.Meta
	}
	file.UpdatedAt = c.now().UTC()

	if err := c.index.Put(r.Context(), file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save file record")
		return
	}

	writeJSON(w, http.StatusOK, file)
}

// Delete removes a file record and its bytes.
func (c *FileController) Delete(w http.ResponseWriter, r *http.Request) {
	file, ok := lookup(w, r, c.index)
	if !ok {
		return
	}

	if err := c.index.Delete(r.Context(), file.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file record")
		return
	}

	if err := c.blobs.Delete(r.Context(), file.Key); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file bytes")
		return
	}

	// The record and bytes are gone; a leftover vector is harmless, so a failure here is logged, not fatal.
	if err := c.vectors.Delete(r.Context(), file.ID); err != nil {
		log.Printf("delete vector for %s: %v", file.ID, err)
	}

	w.WriteHeader(http.StatusNoContent)
}
