package checks

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// candidateLimit is how many of the owner's files the judge sees per claim.
const candidateLimit = int32(3)

// Runner drives one check through its pipeline: split into claims, find candidate files, judge
// each claim, and let the gate decide the verdict. The model proposes; the gate disposes.
type Runner struct {
	store    Store
	index    index.Index
	blobs    blob.Store
	embedder embed.Embedder
	vectors  vectors.Store
	model    Converser
	now      func() time.Time
}

// NewRunner builds a Runner over the stores that already serve the vault.
func NewRunner(store Store, idx index.Index, blobs blob.Store, embedder embed.Embedder, vecs vectors.Store, model Converser) *Runner {
	return &Runner{store: store, index: idx, blobs: blobs, embedder: embedder, vectors: vecs, model: model, now: time.Now}
}

// Run executes the pipeline for one check. A terminal error marks the check failed rather than
// leaving it pending forever; the error is still returned for the caller's logs.
func (r *Runner) Run(ctx context.Context, checkID string, ownerID string) error {
	check, err := r.store.Get(ctx, checkID)
	if err != nil {
		return fmt.Errorf("load check %q: %w", checkID, err)
	}
	if check.OwnerID != ownerID {
		return fmt.Errorf("check %q does not belong to the task's owner", checkID)
	}

	check.Status = domain.CheckRunning
	if err := r.save(ctx, &check); err != nil {
		return err
	}

	if err := r.run(ctx, &check); err != nil {
		check.Status = domain.CheckFailed
		if saveErr := r.save(ctx, &check); saveErr != nil {
			log.Printf("mark check %s failed: %v", check.ID, saveErr)
		}
		return fmt.Errorf("run check %q: %w", check.ID, err)
	}

	check.Status = domain.CheckDone
	return r.save(ctx, &check)
}

// run splits the text and takes every claim through judge and gate.
func (r *Runner) run(ctx context.Context, check *domain.Check) error {
	claims, err := split(ctx, r.model, check.Text)
	if err != nil {
		return err
	}

	for i := range claims {
		r.decide(ctx, check.OwnerID, &claims[i])
	}
	check.Claims = claims
	return nil
}

// decide judges one claim against the owner's candidate files and gates the result. Every path
// out of this function assigns a verdict; the only route to verified runs through the gate.
func (r *Runner) decide(ctx context.Context, ownerID string, claim *domain.Claim) {
	claim.Verdict = domain.VerdictUnsupported

	candidates := r.candidates(ctx, ownerID, claim.Text)
	if len(candidates) == 0 {
		return
	}

	judged, err := judge(ctx, r.model, claim.Text, candidates)
	if err != nil {
		log.Printf("judge claim %q: %v", claim.Text, err)
		return
	}
	if judged.Tier == domain.TierNone || judged.Span == "" {
		return
	}

	cited, ok := findCandidate(candidates, judged.FileID)
	if !ok {
		log.Printf("gate: judge cited unknown file %q for claim %q", judged.FileID, claim.Text)
		return
	}

	// The gate: locate the span in the cited file's stored text and confirm it byte for byte.
	// Offsets come from Locate, never from the model.
	start, end, ok := Locate(cited.Text, judged.Span)
	if !ok || !Verify(cited.Text, judged.Span, start, end) {
		// The judge asserted a span the stored text does not contain: a red flag on the judge,
		// logged loudly, and the claim stays unsupported. It is never softened to review.
		log.Printf("GATE FAIL: judge span not in %s for claim %q: span %q", cited.FileID, claim.Text, judged.Span)
		return
	}

	claim.Reference = &domain.Reference{
		FileID:   cited.FileID,
		FileName: cited.FileName,
		SpanText: judged.Span,
		Start:    start,
		End:      end,
		Tier:     judged.Tier,
		Verified: judged.Tier == domain.TierVerbatim,
	}
	switch judged.Tier {
	case domain.TierVerbatim:
		claim.Verdict = domain.VerdictVerified
	case domain.TierParaphrase:
		claim.Verdict = domain.VerdictReview
	default:
		// An unknown tier is treated as none: the reference is kept for the trace, the verdict
		// stays unsupported.
		claim.Reference.Verified = false
		claim.Verdict = domain.VerdictUnsupported
	}
}

// candidates finds the owner's files most likely to bear on the claim and loads their stored
// text. Files without stored text (dropped before text persistence existed) are skipped.
func (r *Runner) candidates(ctx context.Context, ownerID string, claim string) []candidate {
	vector, err := r.embedder.Embed(ctx, claim)
	if err != nil {
		log.Printf("embed claim %q: %v", claim, err)
		return nil
	}
	ids, err := r.vectors.Query(ctx, ownerID, vector, candidateLimit)
	if err != nil {
		log.Printf("query candidates: %v", err)
		return nil
	}

	loaded := make([]candidate, 0, len(ids))
	for _, id := range ids {
		file, err := r.index.Get(ctx, id)
		if err != nil || file.OwnerID != ownerID {
			continue
		}
		text, _, err := r.blobs.Get(ctx, blob.TextKey(id))
		if err != nil || len(text) == 0 {
			log.Printf("candidate %s has no stored text, skipped (re-drop the file to backfill)", id)
			continue
		}
		loaded = append(loaded, candidate{FileID: file.ID, FileName: file.Name, Text: string(text)})
	}
	return loaded
}

// findCandidate returns the candidate the judge cited.
func findCandidate(candidates []candidate, fileID string) (candidate, bool) {
	for _, c := range candidates {
		if c.FileID == fileID {
			return c, true
		}
	}
	return candidate{}, false
}

// save stamps and persists the check.
func (r *Runner) save(ctx context.Context, check *domain.Check) error {
	check.UpdatedAt = r.now().UTC()
	if err := r.store.Put(ctx, *check); err != nil {
		return fmt.Errorf("save check %q: %w", check.ID, err)
	}
	return nil
}
