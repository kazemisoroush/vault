package controller

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/checks"
	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// maxCheckChars bounds one check's text to roughly a pasted page or two.
const maxCheckChars = 20000

// CheckController creates checks and serves their results.
type CheckController struct {
	store    checks.Store
	enqueuer checks.Enqueuer
	now      func() time.Time
	newID    func() string
}

// NewCheckController builds a check controller with a real clock and id generator.
func NewCheckController(store checks.Store, enqueuer checks.Enqueuer) *CheckController {
	return &CheckController{store: store, enqueuer: enqueuer, now: time.Now, newID: uuid.NewString}
}

// createCheckRequest is the body of a POST /checks call.
type createCheckRequest struct {
	Text string `json:"text"`
}

// Create registers a pending check and hands it to the async worker. The reply is immediate; the
// client polls GET /checks/{id} while the pipeline runs.
func (c *CheckController) Create(w http.ResponseWriter, r *http.Request) {
	var req createCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	// The contract promises a character limit, so count runes, not bytes.
	if utf8.RuneCountInString(req.Text) > maxCheckChars {
		writeError(w, http.StatusBadRequest, "text is too long to check in one go")
		return
	}

	now := c.now().UTC()
	check := domain.Check{
		ID:        c.newID(),
		OwnerID:   auth.OwnerID(r.Context()),
		Text:      req.Text,
		Status:    domain.CheckPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := c.store.Put(r.Context(), check); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save the check")
		return
	}
	if err := c.enqueuer.Enqueue(r.Context(), check.ID, check.OwnerID); err != nil {
		log.Printf("enqueue check %s: %v", check.ID, err)
		// The stored record must not sit pending forever when no worker is coming, so it is
		// marked failed best-effort before the error goes back to the caller.
		check.Status = domain.CheckFailed
		check.UpdatedAt = c.now().UTC()
		if saveErr := c.store.Put(r.Context(), check); saveErr != nil {
			log.Printf("mark unenqueued check %s failed: %v", check.ID, saveErr)
		}
		writeError(w, http.StatusInternalServerError, "could not start the check")
		return
	}

	writeJSON(w, http.StatusAccepted, check)
}

// Get returns one check with its claims and verdicts.
func (c *CheckController) Get(w http.ResponseWriter, r *http.Request) {
	check, err := c.store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, checks.ErrNotFound) {
			writeError(w, http.StatusNotFound, "check not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not read the check")
		return
	}
	if check.OwnerID != auth.OwnerID(r.Context()) {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	writeJSON(w, http.StatusOK, check)
}
