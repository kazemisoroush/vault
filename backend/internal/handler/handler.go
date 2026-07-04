// Package handler exposes the five Vault verbs over HTTP.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

const (
	defaultLimit  = int32(25)
	maxLimit      = int32(100)
	presignExpiry = 15 * time.Minute
)

// Handler routes the API to the index and the blob store.
type Handler struct {
	index index.Index
	blobs blob.Store
	now   func() time.Time
	newID func() string
}

// New builds a Handler with real clock and id generator.
func New(idx index.Index, blobs blob.Store) *Handler {
	return &Handler{
		index: idx,
		blobs: blobs,
		now:   time.Now,
		newID: uuid.NewString,
	}
}

// Routes returns the HTTP mux for all endpoints.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", h.createFile)
	mux.HandleFunc("GET /files", h.listFiles)
	mux.HandleFunc("GET /files/{id}", h.getFile)
	mux.HandleFunc("PATCH /files/{id}", h.updateFile)
	mux.HandleFunc("DELETE /files/{id}", h.deleteFile)
	mux.HandleFunc("GET /health", h.health)

	return mux
}

type createRequest struct {
	Name        string            `json:"name"`
	ContentType string            `json:"contentType"`
	Size        int64             `json:"size"`
	Meta        map[string]string `json:"meta"`
}

type createResponse struct {
	File      domain.File `json:"file"`
	UploadURL string      `json:"uploadUrl"`
}

func (h *Handler) createFile(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.ContentType == "" {
		writeError(w, http.StatusBadRequest, "name and contentType are required")
		return
	}

	now := h.now().UTC()
	file := domain.File{
		ID:          h.newID(),
		Name:        req.Name,
		ContentType: req.ContentType,
		Size:        req.Size,
		Status:      domain.StatusPending,
		Meta:        req.Meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	file.Key = domain.Key(file.ID)

	uploadURL, err := h.blobs.PresignPut(r.Context(), file.Key, file.ContentType, presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not presign upload")
		return
	}

	if err := h.index.Put(r.Context(), file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save file record")
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{File: file, UploadURL: uploadURL})
}

type getResponse struct {
	File        domain.File `json:"file"`
	DownloadURL string      `json:"downloadUrl"`
}

func (h *Handler) getFile(w http.ResponseWriter, r *http.Request) {
	file, ok := h.lookup(w, r)
	if !ok {
		return
	}

	downloadURL, err := h.blobs.PresignGet(r.Context(), file.Key, presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not presign download")
		return
	}

	writeJSON(w, http.StatusOK, getResponse{File: file, DownloadURL: downloadURL})
}

type listResponse struct {
	Files  []domain.File `json:"files"`
	Cursor string        `json:"cursor,omitempty"`
}

func (h *Handler) listFiles(w http.ResponseWriter, r *http.Request) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(int32(parsed), maxLimit)
	}

	files, cursor, err := h.index.List(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list files")
		return
	}

	writeJSON(w, http.StatusOK, listResponse{Files: files, Cursor: cursor})
}

type updateRequest struct {
	Name *string            `json:"name"`
	Meta *map[string]string `json:"meta"`
}

func (h *Handler) updateFile(w http.ResponseWriter, r *http.Request) {
	file, ok := h.lookup(w, r)
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
	file.UpdatedAt = h.now().UTC()

	if err := h.index.Put(r.Context(), file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save file record")
		return
	}

	writeJSON(w, http.StatusOK, file)
}

func (h *Handler) deleteFile(w http.ResponseWriter, r *http.Request) {
	file, ok := h.lookup(w, r)
	if !ok {
		return
	}

	if err := h.index.Delete(r.Context(), file.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file record")
		return
	}

	if err := h.blobs.Delete(r.Context(), file.Key); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file bytes")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) (domain.File, bool) {
	file, err := h.index.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, index.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return domain.File{}, false
		}
		writeError(w, http.StatusInternalServerError, "could not read file record")
		return domain.File{}, false
	}

	return file, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		fmt.Printf("write response: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
