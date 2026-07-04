package transport

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

//go:generate go tool mockgen -source=ingester.go -destination=../mocks/ingester_mock.go -package=mocks

// Ingester processes an S3 object-created event.
type Ingester interface {
	Handle(ctx context.Context, event events.S3Event) error
}
