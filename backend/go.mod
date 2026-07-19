module github.com/kazemisoroush/vault/backend

go 1.24.1

require (
	github.com/MicahParks/keyfunc/v3 v3.8.0
	github.com/anthropics/anthropic-sdk-go v1.56.0
	github.com/aws/aws-lambda-go v1.54.0
	github.com/aws/aws-sdk-go-v2 v1.42.1
	github.com/aws/aws-sdk-go-v2/config v1.32.27
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.20.50
	github.com/aws/aws-sdk-go-v2/service/bedrockagent v1.57.1
	github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime v1.54.1
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.59.2
	github.com/aws/aws-sdk-go-v2/service/lambda v1.97.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.104.2
	github.com/awslabs/aws-lambda-go-api-proxy v0.16.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/mock v0.6.0
	golang.org/x/image v0.23.0
)

require (
	github.com/MicahParks/jwkset v0.11.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.14 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.26 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.34.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.2.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.31.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.43.5 // indirect
	github.com/aws/smithy-go v1.27.3 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool go.uber.org/mock/mockgen
