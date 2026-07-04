package calls

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

const (
	// partition keeps every call under one key so recent calls can be queried newest first.
	partition = "call"
	// retention bounds how long a call is kept, via the table's TTL.
	retention = 30 * 24 * time.Hour
)

// DynamoCalls records LLM calls to a DynamoDB table and lists the recent ones.
type DynamoCalls struct {
	client *dynamodb.Client
	table  string
}

// NewDynamoCalls builds a DynamoCalls over one table.
func NewDynamoCalls(client *dynamodb.Client, table string) *DynamoCalls {
	return &DynamoCalls{client: client, table: table}
}

type item struct {
	PK           string `dynamodbav:"pk"`
	SK           string `dynamodbav:"sk"`
	TTL          int64  `dynamodbav:"ttl"`
	Op           string `dynamodbav:"op"`
	Model        string `dynamodbav:"model"`
	Prompt       string `dynamodbav:"prompt"`
	Reply        string `dynamodbav:"reply"`
	InputTokens  int64  `dynamodbav:"inputTokens"`
	OutputTokens int64  `dynamodbav:"outputTokens"`
	LatencyMS    int64  `dynamodbav:"latencyMs"`
	OK           bool   `dynamodbav:"ok"`
	Error        string `dynamodbav:"error"`
	CreatedAt    string `dynamodbav:"createdAt"`
}

// Record emits the metric line and writes the call to the table. It never fails the caller.
func (c *DynamoCalls) Record(ctx context.Context, call llm.Call) {
	log.Print(emfLine(call))

	stamp := call.CreatedAt.Format(time.RFC3339Nano)
	record := item{
		PK: partition,
		// A random suffix keeps the sort key unique even for calls in the same instant.
		SK:           stamp + "#" + randomSuffix(),
		TTL:          call.CreatedAt.Add(retention).Unix(),
		Op:           call.Op,
		Model:        call.Model,
		Prompt:       call.Prompt,
		Reply:        call.Reply,
		InputTokens:  call.InputTokens,
		OutputTokens: call.OutputTokens,
		LatencyMS:    call.LatencyMS,
		OK:           call.OK,
		Error:        call.Error,
		CreatedAt:    stamp,
	}
	av, err := attributevalue.MarshalMap(record)
	if err != nil {
		log.Printf("record llm call: marshal: %v", err)
		return
	}
	if _, err := c.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(c.table), Item: av}); err != nil {
		log.Printf("record llm call: put: %v", err)
	}
}

// randomSuffix returns a short random hex string for sort-key uniqueness.
func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0"
	}
	return hex.EncodeToString(b[:])
}

// List returns the most recent calls, newest first.
func (c *DynamoCalls) List(ctx context.Context, limit int32) ([]llm.Call, error) {
	out, err := c.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(c.table),
		KeyConditionExpression:    aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: partition}},
		ScanIndexForward:          aws.Bool(false),
		Limit:                     aws.Int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("query llm calls: %w", err)
	}

	var records []item
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &records); err != nil {
		return nil, fmt.Errorf("unmarshal llm calls: %w", err)
	}

	result := make([]llm.Call, 0, len(records))
	for _, record := range records {
		created, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
		result = append(result, llm.Call{
			Op:           record.Op,
			Model:        record.Model,
			Prompt:       record.Prompt,
			Reply:        record.Reply,
			InputTokens:  record.InputTokens,
			OutputTokens: record.OutputTokens,
			LatencyMS:    record.LatencyMS,
			OK:           record.OK,
			Error:        record.Error,
			CreatedAt:    created,
		})
	}
	return result, nil
}
