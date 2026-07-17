package kb_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/kb"
)

type fakeClient struct {
	in      *bedrockagentruntime.RetrieveInput
	results []types.KnowledgeBaseRetrievalResult
}

func (f *fakeClient) Retrieve(_ context.Context, in *bedrockagentruntime.RetrieveInput, _ ...func(*bedrockagentruntime.Options)) (*bedrockagentruntime.RetrieveOutput, error) {
	f.in = in
	return &bedrockagentruntime.RetrieveOutput{RetrievalResults: f.results}, nil
}

func TestSearchUsesHybridSearchAndMapsPassages(t *testing.T) {
	// Arrange
	fake := &fakeClient{results: []types.KnowledgeBaseRetrievalResult{
		{Content: &types.RetrievalResultContent{Text: aws.String("Document No. RA3495037")}, Score: aws.Float64(0.9)},
		{Content: &types.RetrievalResultContent{Text: aws.String("an unrelated line")}, Score: aws.Float64(0.4)},
	}}
	searcher := kb.NewSearcher(fake, "KB123")

	// Act
	got, err := searcher.Search(context.Background(), "what is the passport number", 5)

	// Assert: passages carry their text and score, in order.
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Document No. RA3495037", got[0].Text)
	assert.InDelta(t, 0.9, got[0].Score, 1e-9)

	// Assert: the query uses hybrid search against the given knowledge base, with the limit.
	require.NotNil(t, fake.in)
	assert.Equal(t, "KB123", *fake.in.KnowledgeBaseId)
	cfg := fake.in.RetrievalConfiguration.VectorSearchConfiguration
	assert.Equal(t, types.SearchTypeHybrid, cfg.OverrideSearchType)
	assert.Equal(t, int32(5), *cfg.NumberOfResults)
	assert.Equal(t, "what is the passport number", *fake.in.RetrievalQuery.Text)
}
