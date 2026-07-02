package index

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// ErrNotFound is returned when a file record does not exist.
var ErrNotFound = errors.New("file not found")

// DynamoIndex is the DynamoDB implementation of Index.
type DynamoIndex struct {
	table  string
	client *dynamodb.Client
}

// NewDynamoIndex builds a DynamoIndex for one table.
func NewDynamoIndex(client *dynamodb.Client, table string) *DynamoIndex {
	return &DynamoIndex{table: table, client: client}
}

// Put writes the full file record.
func (d *DynamoIndex) Put(ctx context.Context, file domain.File) error {
	item, err := attributevalue.MarshalMap(file)
	if err != nil {
		return fmt.Errorf("marshal file %q: %w", file.ID, err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.table),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put file %q: %w", file.ID, err)
	}

	return nil
}

// Get reads one file record by id.
func (d *DynamoIndex) Get(ctx context.Context, id string) (domain.File, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.table),
		Key:       idKey(id),
	})
	if err != nil {
		return domain.File{}, fmt.Errorf("get file %q: %w", id, err)
	}
	if out.Item == nil {
		return domain.File{}, fmt.Errorf("get file %q: %w", id, ErrNotFound)
	}

	var file domain.File
	if err := attributevalue.UnmarshalMap(out.Item, &file); err != nil {
		return domain.File{}, fmt.Errorf("unmarshal file %q: %w", id, err)
	}

	return file, nil
}

// List returns one page of file records and the cursor for the next page.
func (d *DynamoIndex) List(ctx context.Context, limit int32, cursor string) ([]domain.File, string, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(d.table),
		Limit:     aws.Int32(limit),
	}
	if cursor != "" {
		id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("decode cursor: %w", err)
		}
		input.ExclusiveStartKey = idKey(id)
	}

	out, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("list files: %w", err)
	}

	files := make([]domain.File, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &files); err != nil {
		return nil, "", fmt.Errorf("unmarshal files: %w", err)
	}

	next := ""
	if out.LastEvaluatedKey != nil {
		var last struct {
			ID string `dynamodbav:"id"`
		}
		if err := attributevalue.UnmarshalMap(out.LastEvaluatedKey, &last); err != nil {
			return nil, "", fmt.Errorf("unmarshal cursor: %w", err)
		}
		next = encodeCursor(last.ID)
	}

	return files, next, nil
}

// Delete removes one file record by id.
func (d *DynamoIndex) Delete(ctx context.Context, id string) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.table),
		Key:       idKey(id),
	})
	if err != nil {
		return fmt.Errorf("delete file %q: %w", id, err)
	}

	return nil
}

func idKey(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: id},
	}
}

func encodeCursor(id string) string {
	return base64.URLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(cursor string) (string, error) {
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("decode base64 cursor: %w", err)
	}

	return string(raw), nil
}
