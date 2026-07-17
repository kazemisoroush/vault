// Package kb searches the managed Bedrock Knowledge Base by hybrid search for the passages
// relevant to a query.
package kb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
)

// Metadata keys the ingestion side stamps on each document and the searcher reads back, so a
// retrieved passage can be tied to the file it came from.
const (
	MetaFileID   = "fileId"
	MetaFileName = "fileName"
)

// client is the slice of the Bedrock agent-runtime API the Searcher uses, kept small to fake in tests.
type client interface {
	Retrieve(ctx context.Context, in *bedrockagentruntime.RetrieveInput, optFns ...func(*bedrockagentruntime.Options)) (*bedrockagentruntime.RetrieveOutput, error)
}

// Searcher finds passages in the Knowledge Base by hybrid search, combining vector similarity with
// keyword (BM25) scoring, so an exact token such as a passport number ranks as well as a paraphrase.
type Searcher struct {
	client client
	kbID   string
}

// NewSearcher builds a Searcher over a Knowledge Base id.
func NewSearcher(client client, kbID string) *Searcher {
	return &Searcher{client: client, kbID: kbID}
}

// Passage is one retrieved chunk: its text, the file it came from (from the document metadata the
// ingestion side stamps), and its relevance score.
type Passage struct {
	Text     string
	FileID   string
	FileName string
	Score    float64
}

// Search returns the passages most relevant to the query by hybrid search, at most limit of them.
func (s *Searcher) Search(ctx context.Context, query string, limit int) ([]Passage, error) {
	out, err := s.client.Retrieve(ctx, &bedrockagentruntime.RetrieveInput{
		KnowledgeBaseId: aws.String(s.kbID),
		RetrievalQuery:  &types.KnowledgeBaseQuery{Text: aws.String(query)},
		RetrievalConfiguration: &types.KnowledgeBaseRetrievalConfiguration{
			VectorSearchConfiguration: &types.KnowledgeBaseVectorSearchConfiguration{
				NumberOfResults:    aws.Int32(int32(limit)),
				OverrideSearchType: types.SearchTypeHybrid,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("kb search: %w", err)
	}

	passages := make([]Passage, 0, len(out.RetrievalResults))
	for _, result := range out.RetrievalResults {
		passage := Passage{
			FileID:   metaString(result.Metadata, MetaFileID),
			FileName: metaString(result.Metadata, MetaFileName),
		}
		if result.Content != nil && result.Content.Text != nil {
			passage.Text = *result.Content.Text
		}
		if result.Score != nil {
			passage.Score = *result.Score
		}
		passages = append(passages, passage)
	}
	return passages, nil
}

// metaString reads a string metadata value the ingestion side stamped, or "" when it is absent or
// not a string.
func metaString(meta map[string]document.Interface, key string) string {
	doc, ok := meta[key]
	if !ok {
		return ""
	}
	var value string
	if err := doc.UnmarshalSmithyDocument(&value); err != nil {
		return ""
	}
	return value
}
