package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// FileController serves the five CRUD verbs over file records and their blobs.
type FileController struct {
	index index.Index
	blobs blob.Store
	now   func() time.Time
	newID func() string
}

// NewFileController builds a file controller with a real clock and id generator.
func NewFileController(idx index.Index, blobs blob.Store) *FileController {
	return &FileController{index: idx, blobs: blobs, now: time.Now, newID: uuid.NewString}
}

// dropRequest is the body of a POST /files call.
type dropRequest struct {
	Name        string            `json:"name"`
	ContentType string            `json:"contentType"`
	Size        int64             `json:"size"`
	Meta        map[string]string `json:"meta"`
}

// dropResponse is the body returned by a POST /files call.
type dropResponse struct {
	File      domain.File `json:"file"`
	UploadURL string      `json:"uploadUrl"`
}

// Drop registers a pending file under a temporary upload id and presigns a PUT to its staging key.
func (c *FileController) Drop(w http.ResponseWriter, r *http.Request) {
	var req dropRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.ContentType == "" {
		writeError(w, http.StatusBadRequest, "name and contentType are required")
		return
	}

	now := c.now().UTC()
	file := domain.File{
		ID:          c.newID(),
		OwnerID:       auth.OwnerID(r.Context()),
		Name:        req.Name,
		ContentType: req.ContentType,
		Size:        req.Size,
		Status:      domain.StatusPending,
		Meta:        req.Meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	file.Key = blob.StagingKey(file.ID)

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

	ownerID := auth.OwnerID(r.Context())
	files, cursor, err := c.index.List(r.Context(), ownerID, limit, r.URL.Query().Get("cursor"))
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

	// Remove the Knowledge Base source and its metadata sidecar too, so the next ingestion sync
	// drops the file from the index. Leftovers are harmless, so a failure here is logged, not fatal.
	if err := c.blobs.Delete(r.Context(), blob.KBKey(file.ID)); err != nil {
		log.Printf("delete kb source for %s: %v", file.ID, err)
	}
	if err := c.blobs.Delete(r.Context(), blob.KBMetadataKey(file.ID)); err != nil {
		log.Printf("delete kb metadata sidecar for %s: %v", file.ID, err)
	}

	w.WriteHeader(http.StatusNoContent)
}
