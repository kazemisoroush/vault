package checks_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/checks"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// contractText and emailText are the stored canonical texts of the two candidate files.
const (
	contractText = "The contract was executed on 14 February 2023. The deposit of $40,000 was payable within seven days."
	emailText    = "We regret to advise the deposit was not paid within seven days; funds cleared on 2 March."
)

// newRunnerFixture wires a Runner whose stores hold one owner, two files with stored text, and
// one pending check. The split is pure code now, so every scripted converser reply is a judge
// reply, one per claim, in claim order.
func newRunnerFixture(t *testing.T, checkText string, judgeReplies ...string) (*checks.Runner, *domain.Check) {
	t.Helper()
	ctrl := gomock.NewController(t)

	store := mocks.NewMockCheckStore(ctrl)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	embedder := mocks.NewMockEmbedder(ctrl)
	vectors := mocks.NewMockVectorStore(ctrl)
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

	// Every claim resolves to the same two candidate files with stored text.
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return([]float32{0.5}, nil).AnyTimes()
	vectors.EXPECT().Query(gomock.Any(), "alice", gomock.Any(), gomock.Any()).Return([]string{"file-1", "file-2"}, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "file-1").Return(domain.File{ID: "file-1", OwnerID: "alice", Name: "Contract of Sale.pdf"}, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "file-2").Return(domain.File{ID: "file-2", OwnerID: "alice", Name: "Email chain.pdf"}, nil).AnyTimes()
	blobs.EXPECT().Get(gomock.Any(), "text/file-1").Return([]byte(contractText), "text/plain", nil).AnyTimes()
	blobs.EXPECT().Get(gomock.Any(), "text/file-2").Return([]byte(emailText), "text/plain", nil).AnyTimes()

	return checks.NewRunner(store, idx, blobs, embedder, vectors, model), check
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

func TestRunLexicalRescuesAnExactValueTheVectorSearchMissed(t *testing.T) {
	// Arrange: the claim keys on an identifier. The vector search ranks only a decoy without the
	// number, but the passport file carries it in metadata, so the literal source must surface the
	// passport for the judge and the claim must find support it would otherwise miss.
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	embedder := mocks.NewMockEmbedder(ctrl)
	vectors := mocks.NewMockVectorStore(ctrl)
	model := mocks.NewMockConverser(ctrl)

	claim := "Soroush Kazemi's Australian passport number is RA3495037."
	check := &domain.Check{ID: "chk-1", OwnerID: "alice", Text: claim, Status: domain.CheckPending}
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(*check, nil)
	store.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c domain.Check) error {
		*check = c
		return nil
	}).AnyTimes()

	// The vector search returns only a decoy file that does not contain the number.
	decoy := domain.File{ID: "license", OwnerID: "alice", Name: "Driving License.jpg"}
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return([]float32{0.5}, nil).AnyTimes()
	vectors.EXPECT().Query(gomock.Any(), "alice", gomock.Any(), gomock.Any()).Return([]string{"license"}, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "license").Return(decoy, nil).AnyTimes()
	blobs.EXPECT().Get(gomock.Any(), "text/license").Return([]byte("a driving license, no passport number here"), "text/plain", nil).AnyTimes()

	// The literal scan lists the owner's files; only the passport carries the number, in its metadata.
	passport := domain.File{ID: "passport", OwnerID: "alice", Name: "IMG_4326.JPG", Meta: map[string]string{"passport_number": "RA3495037"}}
	idx.EXPECT().List(gomock.Any(), "alice", gomock.Any(), "").Return([]domain.File{decoy, passport}, "", nil)
	blobs.EXPECT().Get(gomock.Any(), "text/passport").Return([]byte("Document No. RA3495037 KAZEMI SOROUSH"), "text/plain", nil).AnyTimes()

	// The judge quotes the number from the passport candidate the literal source added.
	model.EXPECT().Converse(gomock.Any(), gomock.Any()).
		Return(`[{"fileId": "passport", "span": "RA3495037", "relation": "paraphrase"}]`, nil)

	runner := checks.NewRunner(store, idx, blobs, embedder, vectors, model)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: the passport, reached only through the literal source, supports the claim.
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictReview, got.Verdict)
	require.Len(t, got.References, 1)
	assert.Equal(t, "passport", got.References[0].FileID)
}

func TestRunRefusesForeignOwner(t *testing.T) {
	// Arrange: the task claims an owner the check does not belong to.
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(domain.Check{ID: "chk-1", OwnerID: "alice"}, nil)

	runner := checks.NewRunner(store, mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl),
		mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl), mocks.NewMockConverser(ctrl))

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

	runner := checks.NewRunner(store, mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl),
		mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl), mocks.NewMockConverser(ctrl))

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
