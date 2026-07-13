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

// candidateLimit is how many of the owner's nearest-by-meaning files the judge sees per claim.
// Chunk-level embeddings rank the right document higher and a literal source now rescues exact
// values, so this window can sit a little wider than the original 3 for better recall without
// leaning on either alone. The gate still verifies every span, so a wider window cannot green a
// claim the evidence does not support.
const candidateLimit = int32(6)

// maxClaims bounds how many sentences one check may spend model calls on. The API's character
// limit admits pathological inputs of thousands of tiny sentences; past this cap the check is
// marked failed rather than driving an unbounded run of Bedrock calls.
const maxClaims = 500

// maxStoredRefs bounds how many references one claim persists. The cap is applied after the
// gate and after the verdict, with contradictions kept first, so truncation can never change
// or hide what the verdict weighed.
const maxStoredRefs = 5

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

// run splits the text and takes every claim through judge and gate. The split is pure code, so
// coverage is total by construction and the pipeline's first model call is the judge.
func (r *Runner) run(ctx context.Context, check *domain.Check) error {
	claims := split(check.Text)
	if len(claims) > maxClaims {
		return fmt.Errorf("check has %d sentences, over the %d cap", len(claims), maxClaims)
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

// decide judges one claim against the owner's candidate files and gates every finding. Every
// path out of this function assigns a verdict; the only route to verified runs through the
// gate, and a gate-verified contradiction outranks everything.
func (r *Runner) decide(ctx context.Context, ownerID string, claim *domain.Claim) {
	claim.Verdict = domain.VerdictUnsupported

	candidates := r.candidates(ctx, ownerID, claim.Text)
	if len(candidates) == 0 {
		return
	}

	findings, err := judge(ctx, r.model, claim.Text, candidates)
	if err != nil {
		// The claim text stays out of the logs: it can carry the user's legal matter.
		log.Printf("judge claim at %d..%d: %v", claim.Start, claim.End, err)
		return
	}

	refs := make([]domain.Reference, 0, len(findings))
	for _, f := range findings {
		if ref, ok := r.gateFinding(claim, f, candidates); ok {
			refs = append(refs, ref)
		}
	}
	claim.Verdict = verdictFor(refs)
	claim.References = capRefs(refs)
}

// capRefs bounds the persisted references, keeping every contradiction first: a truncated
// reference list must still show the evidence that disputed the claim.
func capRefs(refs []domain.Reference) []domain.Reference {
	if len(refs) <= maxStoredRefs {
		return refs
	}
	kept := make([]domain.Reference, 0, maxStoredRefs)
	for _, ref := range refs {
		if ref.Relation == domain.RelationContradicts && len(kept) < maxStoredRefs {
			kept = append(kept, ref)
		}
	}
	for _, ref := range refs {
		if ref.Relation != domain.RelationContradicts && len(kept) < maxStoredRefs {
			kept = append(kept, ref)
		}
	}
	return kept
}

// gateFinding turns one judge finding into a persisted reference, or rejects it. The gate:
// the span must exist in the cited file's stored text character for character, with offsets
// located by code, never taken from the model. A verbatim relation must additionally survive
// the code-level claim-span comparison or it is demoted to paraphrase, which closes the
// injection route where a hostile document steers the judge to call an existing-but-irrelevant
// span "verbatim".
func (r *Runner) gateFinding(claim *domain.Claim, f finding, candidates []candidate) (domain.Reference, bool) {
	if f.Span == "" {
		return domain.Reference{}, false
	}
	relation := f.Relation
	switch relation {
	case domain.RelationVerbatim, domain.RelationParaphrase, domain.RelationContradicts:
	default:
		// An unknown relation is never persisted outside the contract's enum.
		log.Printf("judge returned unknown relation %q for claim at %d..%d", f.Relation, claim.Start, claim.End)
		return domain.Reference{}, false
	}

	cited, ok := findCandidate(candidates, f.FileID)
	if !ok {
		log.Printf("gate: judge cited unknown file %q for claim at %d..%d", f.FileID, claim.Start, claim.End)
		return domain.Reference{}, false
	}

	start, end, ok := Locate(cited.Text, f.Span)
	if !ok || !Verify(cited.Text, f.Span, start, end) {
		// The judge asserted a span the stored text does not contain: a red flag on the judge,
		// logged loudly (ids and lengths only, never the text), and the finding is discarded.
		log.Printf("GATE FAIL: judge span (%d chars) not in %s for claim at %d..%d", len(f.Span), cited.FileID, claim.Start, claim.End)
		return domain.Reference{}, false
	}

	if relation == domain.RelationVerbatim && !verbatimMatches(claim.Text, f.Span) {
		log.Printf("gate: verbatim relation demoted, span does not restate claim at %d..%d", claim.Start, claim.End)
		relation = domain.RelationParaphrase
	}

	return domain.Reference{
		FileID:   cited.FileID,
		FileName: cited.FileName,
		SpanText: f.Span,
		Start:    start,
		End:      end,
		Relation: relation,
	}, true
}

// verdictFor derives the claim's verdict from its gate-verified references, in precedence
// order: any contradiction disputes the claim regardless of support, a code-matched verbatim
// verifies it, reworded support goes to human review, and silence stays unsupported.
func verdictFor(refs []domain.Reference) string {
	verdict := domain.VerdictUnsupported
	for _, ref := range refs {
		switch ref.Relation {
		case domain.RelationContradicts:
			return domain.VerdictDisputed
		case domain.RelationVerbatim:
			verdict = domain.VerdictVerified
		case domain.RelationParaphrase:
			if verdict != domain.VerdictVerified {
				verdict = domain.VerdictReview
			}
		}
	}
	return verdict
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

// candidates gathers every file that may bear on a claim: those nearest by meaning, plus those that
// literally contain a distinctive value from the claim, such as an identifier. The two sources are
// merged and deduplicated by file, so a file found by both appears once. The literal source rescues
// exact-value claims that a meaning search ranks too low to reach, and the gate keeps either source
// from ever producing a false green.
func (r *Runner) candidates(ctx context.Context, ownerID string, claim string) []candidate {
	merged := r.vectorCandidates(ctx, ownerID, claim)
	seen := make(map[string]struct{}, len(merged))
	for _, c := range merged {
		seen[c.FileID] = struct{}{}
	}
	for _, c := range r.lexicalCandidates(ctx, ownerID, claim) {
		if _, ok := seen[c.FileID]; ok {
			continue
		}
		seen[c.FileID] = struct{}{}
		merged = append(merged, c)
	}
	return merged
}

// vectorCandidates finds the owner's files nearest the claim by meaning and loads their stored text.
// Files without stored text (dropped before text persistence existed) are skipped.
func (r *Runner) vectorCandidates(ctx context.Context, ownerID string, claim string) []candidate {
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
