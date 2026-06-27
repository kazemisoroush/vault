package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/kazemisoroush/vault/api/internal/model"
)

const defaultUserID = "USER#default"

const maxBatchWriteRetries = 5

// DynamoRepository implements Repository using DynamoDB.
type DynamoRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoRepository creates a new DynamoDB repository.
func NewDynamoRepository(client *dynamodb.Client, tableName string) *DynamoRepository {
	return &DynamoRepository{client: client, tableName: tableName}
}

// encodeToken serialises a DynamoDB pagination key into an opaque token.
func encodeToken(key map[string]types.AttributeValue) (*string, error) {
	if len(key) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(key))
	for k, v := range key {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			m[k] = s.Value
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encoding next token: %w", err)
	}
	token := base64.StdEncoding.EncodeToString(b)
	return &token, nil
}

// decodeToken restores a DynamoDB pagination key from an opaque token.
func decodeToken(token *string) (map[string]types.AttributeValue, error) {
	if token == nil || *token == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(*token)
	if err != nil {
		return nil, fmt.Errorf("decoding next token: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decoding next token: %w", err)
	}
	key := make(map[string]types.AttributeValue, len(m))
	for k, v := range m {
		key[k] = &types.AttributeValueMemberS{Value: v}
	}
	return key, nil
}

type dynamoItem struct {
	PK           string    `dynamodbav:"PK"`
	SK           string    `dynamodbav:"SK"`
	GSI1PK       string    `dynamodbav:"GSI1PK"`
	GSI1SK       string    `dynamodbav:"GSI1SK"`
	ID           string    `dynamodbav:"id"`
	DriveFileID  string    `dynamodbav:"driveFileId"`
	Name         string    `dynamodbav:"name"`
	NameLower    string    `dynamodbav:"nameLower"`
	MimeType     string    `dynamodbav:"mimeType"`
	Size         int64     `dynamodbav:"size"`
	Category     string    `dynamodbav:"category"`
	Tags         []string  `dynamodbav:"tags"`
	DrivePath    string    `dynamodbav:"drivePath"`
	ThumbnailURL string    `dynamodbav:"thumbnailUrl,omitempty"`
	WebViewURL   string    `dynamodbav:"webViewUrl,omitempty"`
	CreatedAt    time.Time `dynamodbav:"createdAt"`
	UpdatedAt    time.Time `dynamodbav:"updatedAt"`
}

func fileToItem(f model.File) dynamoItem {
	return dynamoItem{
		PK:           defaultUserID,
		SK:           "FILE#" + f.ID,
		GSI1PK:       "CATEGORY#" + string(f.Category),
		GSI1SK:       f.CreatedAt.Format(time.RFC3339),
		ID:           f.ID,
		DriveFileID:  f.DriveFileID,
		Name:         f.Name,
		NameLower:    strings.ToLower(f.Name),
		MimeType:     f.MimeType,
		Size:         f.Size,
		Category:     string(f.Category),
		Tags:         f.Tags,
		DrivePath:    f.DrivePath,
		ThumbnailURL: f.ThumbnailURL,
		WebViewURL:   f.WebViewURL,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
	}
}

func itemToFile(item dynamoItem) model.File {
	return model.File{
		ID:           item.ID,
		DriveFileID:  item.DriveFileID,
		Name:         item.Name,
		MimeType:     item.MimeType,
		Size:         item.Size,
		Category:     model.Category(item.Category),
		Tags:         item.Tags,
		DrivePath:    item.DrivePath,
		ThumbnailURL: item.ThumbnailURL,
		WebViewURL:   item.WebViewURL,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r *DynamoRepository) PutFile(ctx context.Context, file model.File) error {
	item := fileToItem(file)
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshalling file: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &r.tableName,
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("putting file: %w", err)
	}
	return nil
}

func (r *DynamoRepository) GetFile(ctx context.Context, id string) (*model.File, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: defaultUserID},
			"SK": &types.AttributeValueMemberS{Value: "FILE#" + id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting file: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}

	var item dynamoItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshalling file: %w", err)
	}

	file := itemToFile(item)
	return &file, nil
}

func (r *DynamoRepository) DeleteFile(ctx context.Context, id string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: defaultUserID},
			"SK": &types.AttributeValueMemberS{Value: "FILE#" + id},
		},
	})
	if err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}
	return nil
}

func (r *DynamoRepository) ListFiles(ctx context.Context, query model.FileQuery) (*model.FileListResult, error) {
	limit := int32(query.Limit)
	if limit == 0 {
		limit = 50
	}

	if query.Category != nil {
		return r.listByCategory(ctx, *query.Category, limit, query.NextToken)
	}

	startKey, err := decodeToken(query.NextToken)
	if err != nil {
		return nil, err
	}

	input := &dynamodb.QueryInput{
		TableName:              &r.tableName,
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: defaultUserID},
		},
		Limit:            &limit,
		ScanIndexForward:  aws.Bool(false),
		ExclusiveStartKey: startKey,
	}

	var filters []string
	if query.Search != nil && *query.Search != "" {
		searchLower := strings.ToLower(*query.Search)
		filters = append(filters, "contains(nameLower, :search)")
		input.ExpressionAttributeValues[":search"] = &types.AttributeValueMemberS{Value: searchLower}
	}
	if query.Tag != nil && *query.Tag != "" {
		filters = append(filters, "contains(tags, :tag)")
		input.ExpressionAttributeValues[":tag"] = &types.AttributeValueMemberS{Value: *query.Tag}
	}
	if len(filters) > 0 {
		input.FilterExpression = aws.String(strings.Join(filters, " AND "))
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("querying files: %w", err)
	}

	files := make([]model.File, 0, len(result.Items))
	for _, item := range result.Items {
		var di dynamoItem
		if err := attributevalue.UnmarshalMap(item, &di); err != nil {
			return nil, fmt.Errorf("unmarshalling item: %w", err)
		}
		files = append(files, itemToFile(di))
	}

	nextToken, err := encodeToken(result.LastEvaluatedKey)
	if err != nil {
		return nil, err
	}

	return &model.FileListResult{Files: files, NextToken: nextToken}, nil
}

func (r *DynamoRepository) listByCategory(ctx context.Context, category model.Category, limit int32, nextToken *string) (*model.FileListResult, error) {
	startKey, err := decodeToken(nextToken)
	if err != nil {
		return nil, err
	}

	input := &dynamodb.QueryInput{
		TableName:              &r.tableName,
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1PK = :gsi1pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":gsi1pk": &types.AttributeValueMemberS{Value: "CATEGORY#" + string(category)},
		},
		Limit:             &limit,
		ScanIndexForward:  aws.Bool(false),
		ExclusiveStartKey: startKey,
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("querying by category: %w", err)
	}

	files := make([]model.File, 0, len(result.Items))
	for _, item := range result.Items {
		var di dynamoItem
		if err := attributevalue.UnmarshalMap(item, &di); err != nil {
			return nil, fmt.Errorf("unmarshalling item: %w", err)
		}
		files = append(files, itemToFile(di))
	}

	nextToken, err = encodeToken(result.LastEvaluatedKey)
	if err != nil {
		return nil, err
	}

	return &model.FileListResult{Files: files, NextToken: nextToken}, nil
}

func (r *DynamoRepository) ListCategories(ctx context.Context) ([]model.CategoryCount, error) {
	counts := make([]model.CategoryCount, 0, len(model.AllCategories))
	for _, cat := range model.AllCategories {
		input := &dynamodb.QueryInput{
			TableName:              &r.tableName,
			IndexName:              aws.String("GSI1"),
			KeyConditionExpression: aws.String("GSI1PK = :gsi1pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi1pk": &types.AttributeValueMemberS{Value: "CATEGORY#" + string(cat)},
			},
			Select: types.SelectCount,
		}

		result, err := r.client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("counting category %s: %w", cat, err)
		}

		counts = append(counts, model.CategoryCount{
			Category: cat,
			Count:    int(result.Count),
		})
	}
	return counts, nil
}

func (r *DynamoRepository) ListTags(ctx context.Context) ([]string, error) {
	tagSet := make(map[string]struct{})

	input := &dynamodb.QueryInput{
		TableName:              &r.tableName,
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: defaultUserID},
		},
		ProjectionExpression: aws.String("tags"),
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	for _, item := range result.Items {
		var partial struct {
			Tags []string `dynamodbav:"tags"`
		}
		if err := attributevalue.UnmarshalMap(item, &partial); err != nil {
			continue
		}
		for _, tag := range partial.Tags {
			tagSet[tag] = struct{}{}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags, nil
}

func (r *DynamoRepository) BatchPutFiles(ctx context.Context, files []model.File) error {
	for i := 0; i < len(files); i += 25 {
		end := i + 25
		if end > len(files) {
			end = len(files)
		}

		requests := make([]types.WriteRequest, 0, end-i)
		for _, f := range files[i:end] {
			item := fileToItem(f)
			av, err := attributevalue.MarshalMap(item)
			if err != nil {
				return fmt.Errorf("marshalling file %s: %w", f.ID, err)
			}
			requests = append(requests, types.WriteRequest{
				PutRequest: &types.PutRequest{Item: av},
			})
		}

		if err := r.batchWriteWithRetry(ctx, requests); err != nil {
			return err
		}
	}
	return nil
}

// batchWriteWithRetry writes a single batch, retrying UnprocessedItems with backoff.
func (r *DynamoRepository) batchWriteWithRetry(ctx context.Context, requests []types.WriteRequest) error {
	unprocessed := map[string][]types.WriteRequest{r.tableName: requests}

	for attempt := 0; ; attempt++ {
		result, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: unprocessed,
		})
		if err != nil {
			return fmt.Errorf("batch writing files: %w", err)
		}

		unprocessed = result.UnprocessedItems
		if len(unprocessed[r.tableName]) == 0 {
			return nil
		}

		if attempt >= maxBatchWriteRetries {
			return fmt.Errorf("batch writing files: %d items unprocessed after %d retries", len(unprocessed[r.tableName]), attempt)
		}

		backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return fmt.Errorf("batch writing files: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}
}
