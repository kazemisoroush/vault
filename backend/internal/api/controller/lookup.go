package controller

import (
	"errors"
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// lookup loads the file named by the path id, writing the error response on failure.
func lookup(w http.ResponseWriter, r *http.Request, idx index.Index) (domain.File, bool) {
	file, err := idx.Get(r.Context(), r.PathValue("id"))
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
