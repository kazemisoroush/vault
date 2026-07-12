package checks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// DynamoChecks is the DynamoDB implementation of Store.
type DynamoChecks struct {
	table  string
	client *dynamodb.Client
}

// NewDynamoChecks builds a DynamoChecks for one table.
func NewDynamoChecks(client *dynamodb.Client, table string) *DynamoChecks {
	return &DynamoChecks{table: table, client: client}
}

// Put writes the full check record.
func (d *DynamoChecks) Put(ctx context.Context, check domain.Check) error {
	item, err := attributevalue.MarshalMap(check)
	if err != nil {
		return fmt.Errorf("marshal check %q: %w", check.ID, err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.table),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put check %q: %w", check.ID, err)
	}

	return nil
}

// Get reads one check by id.
func (d *DynamoChecks) Get(ctx context.Context, id string) (domain.Check, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.table),
		Key:       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
	})
	if err != nil {
		return domain.Check{}, fmt.Errorf("get check %q: %w", id, err)
	}
	if out.Item == nil {
		return domain.Check{}, fmt.Errorf("get check %q: %w", id, ErrNotFound)
	}

	var check domain.Check
	if err := attributevalue.UnmarshalMap(out.Item, &check); err != nil {
		return domain.Check{}, fmt.Errorf("unmarshal check %q: %w", id, err)
	}

	return check, nil
}
