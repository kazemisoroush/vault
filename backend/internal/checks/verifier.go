package checks

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/kb"
)

// searcher finds the passages relevant to a claim in the Knowledge Base by hybrid search.
// *kb.Searcher satisfies it; the interface lets the verifier be tested with a fake.
type searcher interface {
	Search(ctx context.Context, query string, limit int) ([]kb.Passage, error)
}

// fileIndex reads a stored file by id, so the verifier can drop retrieved candidates whose file the
// claim's owner does not own. *index.DynamoIndex satisfies it; the interface keeps the verifier
// testable with a fake.
type fileIndex interface {
	Get(ctx context.Context, id string) (domain.File, error)
}

// candidateLimit is how many of the owner's files the judge sees per claim.
const candidateLimit = int32(3)

// passageLimit is how many chunk passages hybrid search returns before they are merged by
// file. It exceeds candidateLimit because several passages can come from one file, so the merge
// still leaves candidateLimit distinct files for the judge.
const passageLimit = int32(12)

// maxClaims bounds how many sentences one check may spend model calls on. The API's character
// limit admits pathological inputs of thousands of tiny sentences; past this cap the check is
// marked failed rather than driving an unbounded run of Bedrock calls.
const maxClaims = 500

// maxStoredRefs bounds how many references one claim persists. The cap is applied after the
// gate and after the verdict, with contradictions kept first, so truncation can never change
// or hide what the verdict weighed.
const maxStoredRefs = 5

// Verifier drives one check through its pipeline: split into claims, find candidate files, judge
// each claim, and let the gate decide the verdict. The model proposes; the gate disposes.
type Verifier struct {
	store    Store
	searcher searcher
	files    fileIndex
	model    Converser
	now      func() time.Time
}

// NewVerifier builds a Verifier over the check store, the Knowledge Base searcher, the file index
// that scopes retrieved candidates to their owner, and the model.
func NewVerifier(store Store, s searcher, files fileIndex, model Converser) *Verifier {
	return &Verifier{store: store, searcher: s, files: files, model: model, now: time.Now}
}

// failSaveMargin is the slice of the invocation deadline reserved for marking a check failed
// when the pipeline itself runs out of time, so a timeout cannot strand a check in running.
const failSaveMargin = 10 * time.Second

// Verify executes the pipeline for one check. Once the check is marked running, every way out of
// this function moves it to done or failed: a pipeline error, and the invocation deadline
// itself, both land on failed rather than stranding the check.
func (v *Verifier) Verify(ctx context.Context, checkID string, ownerID string) error {
	check, err := v.store.Get(ctx, checkID)
	if err != nil {
		return fmt.Errorf("load check %q: %w", checkID, err)
	}
	if check.OwnerID != ownerID {
		return fmt.Errorf("check %q does not belong to the task's owner", checkID)
	}

	check.Status = domain.CheckRunning
	if err := v.save(ctx, &check); err != nil {
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

	if err := v.process(runCtx, &check); err != nil {
		check.Status = domain.CheckFailed
		if saveErr := v.save(ctx, &check); saveErr != nil {
			log.Printf("mark check %s failed: %v", check.ID, saveErr)
		}
		return fmt.Errorf("verify check %q: %w", check.ID, err)
	}

	check.Status = domain.CheckDone
	if err := v.save(ctx, &check); err != nil {
		return fmt.Errorf("mark check %q done: %w", check.ID, err)
	}
	return nil
}

// process splits the text and takes every claim through judge and gate. The split is pure code, so
// coverage is total by construction and the pipeline's first model call is the judge.
func (v *Verifier) process(ctx context.Context, check *domain.Check) error {
	claims := split(check.Text)
	if len(claims) > maxClaims {
		return fmt.Errorf("check has %d sentences, over the %d cap", len(claims), maxClaims)
	}

	for i := range claims {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check pipeline interrupted at claim %d of %d: %w", i+1, len(claims), err)
		}
		v.decide(ctx, check.OwnerID, &claims[i])
	}
	check.Claims = claims
	return nil
}

// decide judges one claim against the owner's candidate files and gates every finding. Every
// path out of this function assigns a verdict; the only route to verified runs through the
// gate, and a gate-verified contradiction outranks everything.
func (v *Verifier) decide(ctx context.Context, ownerID string, claim *domain.Claim) {
	claim.Verdict = domain.VerdictUnsupported

	candidates := v.candidates(ctx, ownerID, claim.Text)
	if len(candidates) == 0 {
		return
	}

	findings, err := judge(ctx, v.model, claim.Text, candidates)
	if err != nil {
		// The claim text stays out of the logs: it can carry the user's legal matter.
		log.Printf("judge claim at %d..%d: %v", claim.Start, claim.End, err)
		return
	}

	refs := make([]domain.Reference, 0, len(findings))
	for _, f := range findings {
		if ref, ok := v.gateFinding(claim, f, candidates); ok {
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
func (v *Verifier) gateFinding(claim *domain.Claim, f finding, candidates []candidate) (domain.Reference, bool) {
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

// candidates finds the files most likely to bear on the claim, by hybrid search over the
// Knowledge Base, as the files the judge weighs. Hybrid search returns chunk passages, so several
// passages can come from one file; they are merged by file, joining the chunk text, so the gate
// verifies a cited span against all of that file's retrieved text and not just its first chunk.
// The merged text is what the gate verifies against.
func (v *Verifier) candidates(ctx context.Context, ownerID, claim string) []candidate {
	passages, err := v.searcher.Search(ctx, claim, int(passageLimit))
	if err != nil {
		log.Printf("search candidates: %v", err)
		return nil
	}

	byFile := make(map[string]int)
	merged := make([]candidate, 0, len(passages))
	for _, passage := range passages {
		if passage.Text == "" {
			continue
		}
		if i, ok := byFile[passage.FileID]; ok {
			merged[i].Text += "\n" + passage.Text
			continue
		}
		byFile[passage.FileID] = len(merged)
		merged = append(merged, candidate{FileID: passage.FileID, FileName: passage.FileName, Text: passage.Text})
	}

	// The managed Knowledge Base is one shared store that carries no owner metadata yet, so drop
	// every file the claim's owner does not own before the judge ever sees it. Keep the first
	// candidateLimit owned files.
	owned := make([]candidate, 0, candidateLimit)
	for _, c := range merged {
		if int32(len(owned)) >= candidateLimit {
			break
		}
		file, err := v.files.Get(ctx, c.FileID)
		if err != nil || file.OwnerID != ownerID {
			continue
		}
		owned = append(owned, c)
	}
	return owned
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
func (v *Verifier) save(ctx context.Context, check *domain.Check) error {
	check.UpdatedAt = v.now().UTC()
	if err := v.store.Put(ctx, *check); err != nil {
		return fmt.Errorf("save check %q: %w", check.ID, err)
	}
	return nil
}
