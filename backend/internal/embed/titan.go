package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// titanDimensions is the vector width the index is created with; the query must match it.
const titanDimensions = 1024

// invoker is the slice of the Bedrock runtime client the embedder needs, kept small to fake in tests.
type invoker interface {
	InvokeModel(ctx context.Context, in *bedrockruntime.InvokeModelInput, opts ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

// titanRequest is the Titan embeddings request body.
type titanRequest struct {
	InputText  string `json:"inputText"`
	Dimensions int    `json:"dimensions"`
	Normalize  bool   `json:"normalize"`
}

// titanResponse is the Titan embeddings response body.
type titanResponse struct {
	Embedding           []float32 `json:"embedding"`
	InputTextTokenCount int64     `json:"inputTextTokenCount"`
}

// TitanEmbedder turns text into a vector using Amazon Titan on Bedrock, recording each call.
type TitanEmbedder struct {
	client   invoker
	model    string
	recorder llm.Recorder
}

// NewTitanEmbedder builds a TitanEmbedder for a Bedrock region and model.
func NewTitanEmbedder(ctx context.Context, region, model string, recorder llm.Recorder) (*TitanEmbedder, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &TitanEmbedder{client: bedrockruntime.NewFromConfig(cfg), model: model, recorder: recorder}, nil
}

// Embed returns the vector for a piece of text and records the call to the trace.
func (e *TitanEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()
	body, err := json.Marshal(titanRequest{InputText: text, Dimensions: titanDimensions, Normalize: true})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	out, err := e.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(e.model),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	})
	call := llm.Call{Op: "embed", Model: e.model, Prompt: text, LatencyMS: time.Since(start).Milliseconds(), CreatedAt: start.UTC()}
	if err != nil {
		call.Error = err.Error()
		e.recorder.Record(ctx, call)
		return nil, fmt.Errorf("bedrock embed: %w", err)
	}

	var resp titanResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		call.Error = err.Error()
		e.recorder.Record(ctx, call)
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	call.OK = true
	call.Reply = fmt.Sprintf("[%d-dim vector]", len(resp.Embedding))
	call.InputTokens = resp.InputTextTokenCount
	e.recorder.Record(ctx, call)
	return resp.Embedding, nil
}
