package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/kazemisoroush/vault/api/internal/metadata"
	"github.com/kazemisoroush/vault/api/internal/model"
)

// Handler holds HTTP handlers for the vault API.
type Handler struct {
	service *metadata.Service
}

// NewHandler creates a new handler.
func NewHandler(service *metadata.Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/files", h.ListFiles)
	mux.HandleFunc("GET /api/files/{id}", h.GetFile)
	mux.HandleFunc("POST /api/files", h.UploadFile)
	mux.HandleFunc("PUT /api/files/{id}", h.UpdateFile)
	mux.HandleFunc("DELETE /api/files/{id}", h.DeleteFile)
	mux.HandleFunc("POST /api/sync", h.Sync)
	mux.HandleFunc("GET /api/categories", h.ListCategories)
	mux.HandleFunc("GET /api/tags", h.ListTags)
}

func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	query := model.FileQuery{
		Limit: 50,
	}

	if cat := r.URL.Query().Get("category"); cat != "" {
		c := model.Category(cat)
		query.Category = &c
	}
	if tag := r.URL.Query().Get("tag"); tag != "" {
		query.Tag = &tag
	}
	if search := r.URL.Query().Get("search"); search != "" {
		query.Search = &search
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			query.Limit = l
		}
	}
	if token := r.URL.Query().Get("nextToken"); token != "" {
		query.NextToken = &token
	}

	result, err := h.service.ListFiles(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	file, err := h.service.GetFile(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if file == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	writeJSON(w, http.StatusOK, file)
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer func() { _ = file.Close() }()

	category := model.Category(r.FormValue("category"))
	if !category.IsValid() {
		category = model.CategoryOther
	}

	var tags []string
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			tags = []string{tagsStr}
		}
	}

	req := metadata.UploadRequest{
		Name:     header.Filename,
		MimeType: header.Header.Get("Content-Type"),
		FolderID: r.FormValue("folderId"),
		Category: category,
		Tags:     tags,
		Content:  file,
	}

	result, err := h.service.UploadFile(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req metadata.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.UpdateFile(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.service.DeleteFile(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	count, err := h.service.Sync(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"synced": count})
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, categories)
}

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.service.ListTags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, tags)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
