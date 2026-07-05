package embed

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// fakeInvoker returns a canned Bedrock response body.
type fakeInvoker struct {
	body string
}

func (f fakeInvoker) InvokeModel(_ context.Context, _ *bedrockruntime.InvokeModelInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	return &bedrockruntime.InvokeModelOutput{Body: []byte(f.body)}, nil
}

func TestEmbedReturnsVectorAndRecords(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	recorder := mocks.NewMockRecorder(ctrl)
	recorder.EXPECT().Record(gomock.Any(), gomock.Any())
	embedder := &TitanEmbedder{
		client:   fakeInvoker{body: `{"embedding":[0.1,0.2,0.3],"inputTextTokenCount":4}`},
		model:    "amazon.titan-embed-text-v2:0",
		recorder: recorder,
	}

	// Act
	vec, err := embedder.Embed(context.Background(), "petrol receipt")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vec)
}
