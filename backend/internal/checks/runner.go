package checks

import (
	"context"
	"fmt"
	"log"
	"strings"
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

// failSaveMargin is the slice of the invocation deadline reserved for marking a check failed
// when the pipeline itself runs out of time, so a timeout cannot strand a check in running.
const failSaveMargin = 10 * time.Second

// Run executes the pipeline for one check. Once the check is marked running, every way out of
// this function moves it to done or failed: a pipeline error, and the invocation deadline
// itself, both land on failed rather than stranding the check.
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
		return fmt.Errorf("mark check %q running: %w", check.ID, err)
	}

	// The pipeline gets the invocation deadline minus a margin, so when it times out there is
	// still time on the parent context to record the failure.
	runCtx := ctx
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithDeadline(ctx, deadline.Add(-failSaveMargin))
		defer cancel()
	}

	if err := r.run(runCtx, &check); err != nil {
		check.Status = domain.CheckFailed
		if saveErr := r.save(ctx, &check); saveErr != nil {
			log.Printf("mark check %s failed: %v", check.ID, saveErr)
		}
		return fmt.Errorf("run check %q: %w", check.ID, err)
	}

	check.Status = domain.CheckDone
	if err := r.save(ctx, &check); err != nil {
		return fmt.Errorf("mark check %q done: %w", check.ID, err)
	}
	return nil
}

// run splits the text and takes every claim through judge and gate.
func (r *Runner) run(ctx context.Context, check *domain.Check) error {
	claims, err := split(ctx, r.model, check.Text)
	if err != nil {
		return fmt.Errorf("split check text: %w", err)
	}

	for i := range claims {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check pipeline interrupted at claim %d of %d: %w", i+1, len(claims), err)
		}
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
		// The claim text stays out of the logs: it can carry the user's legal matter.
		log.Printf("judge claim at %d..%d: %v", claim.Start, claim.End, err)
		return
	}
	if judged.Tier == domain.TierNone || judged.Span == "" {
		return
	}

	cited, ok := findCandidate(candidates, judged.FileID)
	if !ok {
		log.Printf("gate: judge cited unknown file %q for claim at %d..%d", judged.FileID, claim.Start, claim.End)
		return
	}

	// The gate: locate the span in the cited file's stored text and confirm it byte for byte.
	// Offsets come from Locate, never from the model.
	start, end, ok := Locate(cited.Text, judged.Span)
	if !ok || !Verify(cited.Text, judged.Span, start, end) {
		// The judge asserted a span the stored text does not contain: a red flag on the judge,
		// logged loudly (ids and lengths only, never the text), and the claim stays
		// unsupported. It is never softened to review.
		log.Printf("GATE FAIL: judge span (%d chars) not in %s for claim at %d..%d", len(judged.Span), cited.FileID, claim.Start, claim.End)
		return
	}

	// A verbatim tier must also survive the claim-span match: the span must actually restate
	// the claim, judged by code, not by the model. This closes the injection route where a
	// hostile document steers the judge to call an existing-but-irrelevant span "verbatim";
	// such a span is demoted to a paraphrase and lands on a human, never on a green.
	tier := judged.Tier
	if tier == domain.TierVerbatim && !verbatimMatches(claim.Text, judged.Span) {
		log.Printf("gate: verbatim tier demoted, span does not restate claim at %d..%d", claim.Start, claim.End)
		tier = domain.TierParaphrase
	}

	switch tier {
	case domain.TierVerbatim, domain.TierParaphrase:
		claim.Reference = &domain.Reference{
			FileID:   cited.FileID,
			FileName: cited.FileName,
			SpanText: judged.Span,
			Start:    start,
			End:      end,
			Tier:     tier,
			Verified: tier == domain.TierVerbatim,
		}
		if tier == domain.TierVerbatim {
			claim.Verdict = domain.VerdictVerified
		} else {
			claim.Verdict = domain.VerdictReview
		}
	default:
		// An unknown tier stays out of the stored reference entirely, so nothing outside the
		// contract's enum is ever persisted; the verdict stays unsupported.
		log.Printf("judge returned unknown tier %q for claim at %d..%d", judged.Tier, claim.Start, claim.End)
	}
}

// verbatimMatches reports whether the span restates the claim closely enough to be called
// verbatim: equal after collapsing runs of whitespace, ignoring case and surrounding
// punctuation. Anything looser is a paraphrase and belongs to a human.
func verbatimMatches(claim string, span string) bool {
	return normalizeForMatch(claim) == normalizeForMatch(span)
}

// normalizeForMatch canonicalises text for the verbatim claim-span comparison only. The gate's
// character-for-character check against the stored document is untouched by this.
func normalizeForMatch(text string) string {
	collapsed := strings.Join(strings.Fields(text), " ")
	return strings.ToLower(strings.Trim(collapsed, " .,;:!?\"'"))
}

// candidates finds the owner's files most likely to bear on the claim and loads their stored
// text. Files without stored text (dropped before text persistence existed) are skipped.
func (r *Runner) candidates(ctx context.Context, ownerID string, claim string) []candidate {
	vector, err := r.embedder.Embed(ctx, claim)
	if err != nil {
		log.Printf("embed claim (%d chars): %v", len(claim), err)
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
