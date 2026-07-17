package transport_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/mocks"
	"github.com/kazemisoroush/vault/backend/internal/transport"
)

const (
	s3Payload  = `{"Records":[{"eventSource":"aws:s3","s3":{"object":{"key":"files/abc"}}}]}`
	apiPayload = `{"version":"2.0","routeKey":"GET /files","rawPath":"/files","requestContext":{"http":{"method":"GET","path":"/files"}}}`
)

func TestHandleRoutesS3ToIngester(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	ingester := mocks.NewMockIngester(ctrl)
	var gotKey string
	ingester.EXPECT().Handle(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e events.S3Event) error {
		gotKey = e.Records[0].S3.Object.Key
		return nil
	})
	proxyCalled := false
	proxy := func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		proxyCalled = true
		return events.APIGatewayV2HTTPResponse{}, nil
	}
	adapter := transport.NewTransport(proxy, ingester, stubVerifier{})

	// Act
	_, err := adapter.Handle(context.Background(), json.RawMessage(s3Payload))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "files/abc", gotKey)
	assert.False(t, proxyCalled, "S3 event must not hit the HTTP proxy")
}

func TestHandleRoutesAPIToProxy(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	ingester := mocks.NewMockIngester(ctrl)
	proxyCalled := false
	proxy := func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		proxyCalled = true
		return events.APIGatewayV2HTTPResponse{StatusCode: 200}, nil
	}
	adapter := transport.NewTransport(proxy, ingester, stubVerifier{})

	// Act
	resp, err := adapter.Handle(context.Background(), json.RawMessage(apiPayload))

	// Assert
	require.NoError(t, err)
	assert.True(t, proxyCalled, "API event must hit the HTTP proxy")
	assert.Equal(t, 200, resp.(events.APIGatewayV2HTTPResponse).StatusCode)
}

// stubVerifier satisfies transport.CheckVerifier for tests that never route a check task.
type stubVerifier struct {
	verify func(ctx context.Context, checkID string, ownerID string) error
}

func (s stubVerifier) Verify(ctx context.Context, checkID string, ownerID string) error {
	if s.verify == nil {
		return nil
	}
	return s.verify(ctx, checkID, ownerID)
}
