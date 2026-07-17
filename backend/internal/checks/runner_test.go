package checks_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/checks"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/kb"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// errNoFile stands in for the file index reporting a missing file.
var errNoFile = errors.New("file not found")

// fakeIndex looks up seeded files by id, standing in for the file index the runner uses to scope
// retrieved candidates to their owner.
type fakeIndex struct{ files map[string]domain.File }

func (f fakeIndex) Get(_ context.Context, id string) (domain.File, error) {
	file, ok := f.files[id]
	if !ok {
		return domain.File{}, errNoFile
	}
	return file, nil
}

// contractText and emailText are the passages the retriever returns for the two candidate files.
const (
	contractText = "The contract was executed on 14 February 2023. The deposit of $40,000 was payable within seven days."
	emailText    = "We regret to advise the deposit was not paid within seven days; funds cleared on 2 March."
)

// fakeRetriever returns fixed passages, standing in for hybrid retrieval over the Knowledge Base.
type fakeRetriever struct{ passages []kb.Passage }

func (f fakeRetriever) Retrieve(_ context.Context, _ string, _ int) ([]kb.Passage, error) {
	return f.passages, nil
}

// newRunnerFixture wires a Runner whose retriever returns two candidate passages and one pending
// check. The split is pure code, so every scripted converser reply is a judge reply, one per claim.
func newRunnerFixture(t *testing.T, checkText string, judgeReplies ...string) (*checks.Runner, *domain.Check) {
	t.Helper()
	ctrl := gomock.NewController(t)

	store := mocks.NewMockCheckStore(ctrl)
	model := mocks.NewMockConverser(ctrl)

	check := &domain.Check{ID: "chk-1", OwnerID: "alice", Text: checkText, Status: domain.CheckPending}
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(*check, nil)
	store.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c domain.Check) error {
		*check = c
		return nil
	}).AnyTimes()

	for _, reply := range judgeReplies {
		model.EXPECT().Converse(gomock.Any(), gomock.Any()).Return(reply, nil)
	}

	// Every claim resolves to the same two candidate passages.
	retriever := fakeRetriever{passages: []kb.Passage{
		{FileID: "file-1", FileName: "Contract of Sale.pdf", Text: contractText},
		{FileID: "file-2", FileName: "Email chain.pdf", Text: emailText},
	}}

	// Both candidate files belong to the check's owner, so neither is scoped out.
	index := fakeIndex{files: map[string]domain.File{
		"file-1": {ID: "file-1", OwnerID: "alice"},
		"file-2": {ID: "file-2", OwnerID: "alice"},
	}}

	return checks.NewRunner(store, retriever, index, model), check
}

func TestRunVerifiesVerbatimSpanThroughTheGate(t *testing.T) {
	// Arrange: one claim; the judge cites a span that truly exists and restates it.
	claim := "The deposit of $40,000 was payable within seven days."
	runner, check := newRunnerFixture(t, claim,
		`[{"fileId": "file-1", "span": "The deposit of $40,000 was payable within seven days.", "relation": "verbatim"}]`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: verified, with offsets located by code in the stored text.
	require.Equal(t, domain.CheckDone, check.Status)
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictVerified, got.Verdict)
	require.Len(t, got.References, 1)
	assert.Equal(t, got.References[0].SpanText, contractText[got.References[0].Start:got.References[0].End])
}

func TestRunContradictionDisputesEvenWithExactSupport(t *testing.T) {
	// Arrange: one file supports the claim verbatim; another contradicts it. Disputed must
	// outrank the green, with both passages kept, because showing a green while holding
	// contradicting evidence would be lying by omission.
	claim := "The deposit of $40,000 was payable within seven days."
	runner, check := newRunnerFixture(t, claim,
		`[{"fileId": "file-1", "span": "The deposit of $40,000 was payable within seven days.", "relation": "verbatim"},
		  {"fileId": "file-2", "span": "the deposit was not paid within seven days", "relation": "contradicts"}]`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: disputed, both references persisted, each gate-verified against its own file.
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictDisputed, got.Verdict)
	require.Len(t, got.References, 2)
	assert.Equal(t, domain.RelationVerbatim, got.References[0].Relation)
	assert.Equal(t, domain.RelationContradicts, got.References[1].Relation)
	assert.Equal(t, got.References[1].SpanText, emailText[got.References[1].Start:got.References[1].End])
}

func TestRunLyingJudgeSpanFailsTheGateAndIsDiscarded(t *testing.T) {
	// Arrange: the judge asserts a span no stored text contains. The gate must discard it and
	// the claim stays unsupported, never softened.
	claim := "The parties agreed to waive the penalty clause."
	runner, check := newRunnerFixture(t, claim,
		`[{"fileId": "file-1", "span": "the parties waived the penalty clause", "relation": "verbatim"}]`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: no references, no green. Zero false greens is the north star.
	require.Len(t, check.Claims, 1)
	assert.Equal(t, domain.VerdictUnsupported, check.Claims[0].Verdict)
	assert.Empty(t, check.Claims[0].References)
}

func TestRunVerbatimRelationDemotedWhenSpanDoesNotRestateClaim(t *testing.T) {
	// Arrange: an injected or confused judge labels an existing but irrelevant span "verbatim".
	// The span passes the existence gate, but it does not restate the claim, so the code-level
	// claim-span match must demote it to review. This is the prompt-injection defence.
	claim := "The parties agreed to waive the penalty clause."
	runner, check := newRunnerFixture(t, claim,
		`[{"fileId": "file-1", "span": "The contract was executed on 14 February 2023.", "relation": "verbatim"}]`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: demoted to review, never verified.
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictReview, got.Verdict)
	require.Len(t, got.References, 1)
	assert.Equal(t, domain.RelationParaphrase, got.References[0].Relation)
}

func TestRunParaphraseLandsInReviewNotVerified(t *testing.T) {
	// Arrange: the span exists but the judge calls it a paraphrase, so a human confirms.
	claim := "The agreement was signed in mid February 2023."
	runner, check := newRunnerFixture(t, claim,
		`[{"fileId": "file-1", "span": "executed on 14 February 2023", "relation": "paraphrase"}]`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert
	require.Len(t, check.Claims, 1)
	assert.Equal(t, domain.VerdictReview, check.Claims[0].Verdict)
}

func TestRunEmptyFindingsStayUnsupported(t *testing.T) {
	// Arrange
	claim := "The tenant kept a pet alpaca on the premises."
	runner, check := newRunnerFixture(t, claim, `[]`)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert
	require.Len(t, check.Claims, 1)
	assert.Equal(t, domain.VerdictUnsupported, check.Claims[0].Verdict)
}

func TestRunDropsForeignOwnerCandidate(t *testing.T) {
	// Arrange: the only retrieved passage belongs to another owner, so it must be scoped out
	// before the judge runs. The model is never expected to be called, so a judge call on the
	// foreign file would fail the test.
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	model := mocks.NewMockConverser(ctrl)

	check := domain.Check{ID: "chk-1", OwnerID: "alice", Text: "The deposit was paid.", Status: domain.CheckPending}
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(check, nil)
	store.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c domain.Check) error {
		check = c
		return nil
	}).AnyTimes()

	retriever := fakeRetriever{passages: []kb.Passage{
		{FileID: "file-2", FileName: "Someone else.pdf", Text: emailText},
	}}
	index := fakeIndex{files: map[string]domain.File{
		"file-2": {ID: "file-2", OwnerID: "mallory"},
	}}
	runner := checks.NewRunner(store, retriever, index, model)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: no owned candidate, so the claim stays unsupported with no references.
	require.Len(t, check.Claims, 1)
	assert.Equal(t, domain.VerdictUnsupported, check.Claims[0].Verdict)
	assert.Empty(t, check.Claims[0].References)
}

func TestRunRefusesForeignOwner(t *testing.T) {
	// Arrange: the task claims an owner the check does not belong to.
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(domain.Check{ID: "chk-1", OwnerID: "alice"}, nil)

	runner := checks.NewRunner(store, fakeRetriever{}, fakeIndex{}, mocks.NewMockConverser(ctrl))

	// Act
	err := runner.Run(context.Background(), "chk-1", "mallory")

	// Assert: refused before any model call or store write.
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "owner"))
}

func TestRunExpiredDeadlineMarksCheckFailed(t *testing.T) {
	// Arrange: the invocation deadline has already passed when the pipeline starts, so the
	// per-claim context check must stop the run and the check must end failed, never running.
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)

	check := domain.Check{ID: "chk-1", OwnerID: "alice", Text: "The deposit was paid.", Status: domain.CheckPending}
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(check, nil)
	var statuses []string
	store.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c domain.Check) error {
		statuses = append(statuses, c.Status)
		return nil
	}).AnyTimes()

	runner := checks.NewRunner(store, fakeRetriever{}, fakeIndex{}, mocks.NewMockConverser(ctrl))

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	// Act
	err := runner.Run(ctx, "chk-1", "alice")

	// Assert: the check ends failed, not stuck pending or running.
	require.Error(t, err)
	assert.Equal(t, []string{domain.CheckRunning, domain.CheckFailed}, statuses)
}

func TestRunContradictionBeyondStorageCapStillDisputes(t *testing.T) {
	// Arrange: six findings, five paraphrases then one contradiction. The verdict must weigh
	// ALL gated findings (disputed), and the stored references must keep the contradiction
	// even though the list is capped.
	claim := "The deposit of $40,000 was payable within seven days."
	findings := `[
	  {"fileId": "file-1", "span": "The contract was executed on 14 February 2023", "relation": "paraphrase"},
	  {"fileId": "file-1", "span": "The deposit of $40,000", "relation": "paraphrase"},
	  {"fileId": "file-1", "span": "payable within seven days", "relation": "paraphrase"},
	  {"fileId": "file-1", "span": "executed on 14 February", "relation": "paraphrase"},
	  {"fileId": "file-1", "span": "14 February 2023", "relation": "paraphrase"},
	  {"fileId": "file-2", "span": "the deposit was not paid within seven days", "relation": "contradicts"}]`
	runner, check := newRunnerFixture(t, claim, findings)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictDisputed, got.Verdict)
	require.Len(t, got.References, 5, "stored references are capped")
	assert.Equal(t, domain.RelationContradicts, got.References[0].Relation, "the contradiction is kept first")
}
