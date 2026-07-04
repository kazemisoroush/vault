package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// Drop registers a file record and returns a presigned upload URL.
type Drop struct {
	index index.Index
	blobs blob.Store
	now   func() time.Time
	newID func() string
}

// NewDrop builds a Drop controller with a real clock and id generator.
func NewDrop(idx index.Index, blobs blob.Store) *Drop {
	return &Drop{index: idx, blobs: blobs, now: time.Now, newID: uuid.NewString}
}

func (c *Drop) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
