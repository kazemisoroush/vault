package checks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/checks"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// contractText is the stored canonical text of the one candidate file in these tests.
const contractText = "The contract was executed on 14 February 2023. The deposit of $40,000 was payable within seven days."

// newRunnerFixture wires a Runner whose stores hold one owner, one file, and one pending check.
// The converser replies are scripted per test: the first call is the splitter, the rest judges.
func newRunnerFixture(t *testing.T, checkText string, replies ...string) (*checks.Runner, *domain.Check, *mocks.MockCheckStore) {
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

	for _, reply := range replies {
		model.EXPECT().Converse(gomock.Any(), gomock.Any()).Return(reply, nil)
	}

	// Every claim resolves to the same single candidate file with stored text.
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return([]float32{0.5}, nil).AnyTimes()
	vectors.EXPECT().Query(gomock.Any(), "alice", gomock.Any(), gomock.Any()).Return([]string{"file-1"}, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "file-1").Return(domain.File{ID: "file-1", OwnerID: "alice", Name: "Contract of Sale.pdf"}, nil).AnyTimes()
	blobs.EXPECT().Get(gomock.Any(), "text/file-1").Return([]byte(contractText), "text/plain", nil).AnyTimes()

	return checks.NewRunner(store, idx, blobs, embedder, vectors, model), check, store
}

func TestRunVerifiesVerbatimSpanThroughTheGate(t *testing.T) {
	// Arrange: one claim; the judge cites a span that truly exists, tier verbatim.
	claim := "The deposit of $40,000 was payable within seven days."
	runner, check, _ := newRunnerFixture(t, claim,
		`["`+claim+`"]`,
		`{"fileId": "file-1", "span": "The deposit of $40,000 was payable within seven days.", "tier": "verbatim"}`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: verified, with offsets located by code in the stored text.
	require.Equal(t, domain.CheckDone, check.Status)
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictVerified, got.Verdict)
	require.NotNil(t, got.Reference)
	assert.True(t, got.Reference.Verified)
	assert.Equal(t, got.Reference.SpanText, contractText[got.Reference.Start:got.Reference.End])
}

func TestRunLyingJudgeSpanFailsTheGateAndStaysUnsupported(t *testing.T) {
	// Arrange: the judge asserts a span the stored text does not contain. The gate must catch
	// it and the verdict must stay unsupported, never softened to review.
	claim := "The parties agreed to waive the penalty clause."
	runner, check, _ := newRunnerFixture(t, claim,
		`["`+claim+`"]`,
		`{"fileId": "file-1", "span": "the parties waived the penalty clause", "tier": "verbatim"}`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert: no reference, no green. Zero false greens is the north star.
	require.Len(t, check.Claims, 1)
	assert.Equal(t, domain.VerdictUnsupported, check.Claims[0].Verdict)
	assert.Nil(t, check.Claims[0].Reference)
}

func TestRunParaphraseLandsInReviewNotVerified(t *testing.T) {
	// Arrange: the span exists but the judge calls it a paraphrase, so only a human may confirm.
	claim := "The agreement was signed in mid February 2023."
	runner, check, _ := newRunnerFixture(t, claim,
		`["`+claim+`"]`,
		`{"fileId": "file-1", "span": "executed on 14 February 2023", "tier": "paraphrase"}`,
	)

	// Act
	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	// Assert
	require.Len(t, check.Claims, 1)
	got := check.Claims[0]
	assert.Equal(t, domain.VerdictReview, got.Verdict)
	require.NotNil(t, got.Reference)
	assert.False(t, got.Reference.Verified, "a paraphrase is never a machine green")
}

func TestRunTierNoneStaysUnsupported(t *testing.T) {
	claim := "The tenant kept a pet alpaca on the premises."
	runner, check, _ := newRunnerFixture(t, claim,
		`["`+claim+`"]`,
		`{"tier": "none"}`,
	)

	require.NoError(t, runner.Run(context.Background(), "chk-1", "alice"))

	require.Len(t, check.Claims, 1)
	assert.Equal(t, domain.VerdictUnsupported, check.Claims[0].Verdict)
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

func TestRunSplitterFailureMarksCheckFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	model := mocks.NewMockConverser(ctrl)

	check := domain.Check{ID: "chk-1", OwnerID: "alice", Text: "some text", Status: domain.CheckPending}
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(check, nil)
	var statuses []string
	store.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c domain.Check) error {
		statuses = append(statuses, c.Status)
		return nil
	}).AnyTimes()
	model.EXPECT().Converse(gomock.Any(), gomock.Any()).Return("", llm.NewRetryableError(assert.AnError))

	runner := checks.NewRunner(store, mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl),
		mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl), model)

	// Act
	err := runner.Run(context.Background(), "chk-1", "alice")

	// Assert: the check ends failed, not stuck pending or running.
	require.Error(t, err)
	assert.Equal(t, []string{domain.CheckRunning, domain.CheckFailed}, statuses)
}
