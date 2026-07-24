package controller

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// maxFields caps how many fields one form may ask for, bounding cost and keeping the whole fill
// under the API Gateway response window.
const maxFields = 50

// fillConcurrency bounds how many fields are answered at once. Each field is a full agent run, so
// answering them in parallel keeps a long form under the 29s API Gateway timeout, while the limit
// stays polite to the Bedrock request rate.
// ponytail: fixed limit of 5; make it configurable only if forms or the Bedrock quota change.
const fillConcurrency = 5

// FillController answers a list of form fields from the owner's vault, each with the file it came
// from, so a long form gets filled from records instead of by hand. It reuses the ask agent: one
// field is one query.
type FillController struct {
	agent agent.Answerer
	blobs blob.Store
}

// NewFillController builds a FillController.
func NewFillController(answerer agent.Answerer, blobs blob.Store) *FillController {
	return &FillController{agent: answerer, blobs: blobs}
}

type fillRequest struct {
	Fields []string `json:"fields"`
}

type fillSource struct {
	File        domain.File `json:"file"`
	DownloadURL string      `json:"downloadUrl"`
}

// fillAnswer is one row of the filled form. found is false, and value empty, when the vault holds
// no sourced answer: these values go on a real form, so an uncited guess is never returned.
type fillAnswer struct {
	Field   string       `json:"field"`
	Value   string       `json:"value"`
	Found   bool         `json:"found"`
	Sources []fillSource `json:"sources"`
}

type fillResponse struct {
	Answers []fillAnswer `json:"answers"`
}

// Fill answers each field against the owner's vault. Fields are answered concurrently but the
// response preserves request order, so the caller can line answers up against the form. A field the
// agent cannot source (no cited file) comes back found:false with an empty value; the caller
// reviews before anything goes on the form.
func (c *FillController) Fill(w http.ResponseWriter, r *http.Request) {
	var req fillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	fields := trimFields(req.Fields)
	if len(fields) == 0 {
		writeError(w, http.StatusBadRequest, "at least one field is required")
		return
	}
	if len(fields) > maxFields {
		writeError(w, http.StatusBadRequest, "too many fields")
		return
	}

	ownerID := auth.OwnerID(r.Context())
	answers := make([]fillAnswer, len(fields))

	g, ctx := errgroup.WithContext(r.Context())
	g.SetLimit(fillConcurrency)
	for i, field := range fields {
		g.Go(func() error {
			answers[i] = c.answerField(ctx, ownerID, field)
			return nil // one field never cancels the rest; a miss is a not-found row, not a failure
		})
	}
	_ = g.Wait()

	writeJSON(w, http.StatusOK, fillResponse{Answers: answers})
}

// answerField runs one field through the agent and presigns each source it cited. A field with no
// cited file, or one the agent errors on, is returned as not found: the form must never carry a
// value the vault could not back with a document.
func (c *FillController) answerField(ctx context.Context, ownerID, field string) fillAnswer {
	result, err := c.agent.Answer(ctx, ownerID, field)
	if err != nil {
		log.Printf("fill: answer %q: %v", field, err)
		return fillAnswer{Field: field, Sources: []fillSource{}}
	}
	if len(result.Files) == 0 {
		return fillAnswer{Field: field, Sources: []fillSource{}}
	}

	sources := make([]fillSource, 0, len(result.Files))
	for _, file := range result.Files {
		downloadURL, err := c.blobs.PresignGet(ctx, file.Key, presignExpiry)
		if err != nil {
			log.Printf("fill: presign %q: %v", file.Key, err)
			continue
		}
		sources = append(sources, fillSource{File: file, DownloadURL: downloadURL})
	}
	return fillAnswer{Field: field, Value: result.Text, Found: true, Sources: sources}
}

// trimFields drops blank field names and trims whitespace, so a stray empty string in the list is
// not answered as a field.
func trimFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
