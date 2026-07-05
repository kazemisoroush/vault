// CDK app defining the Vault walking skeleton stack.
package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2authorizers"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3notifications"
	golambda "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdklabs/cdk-nag-go/cdknag/v2"
)

// extractModel is the Bedrock Claude inference profile that fills metadata on drop.
const extractModel = "us.anthropic.claude-haiku-4-5-20251001-v1:0"

// rerankModel re-ranks the vector shortlist on ask; the same small Claude model as extract.
const rerankModel = extractModel

// embedModel turns text into a vector for search, on both drop and ask.
const embedModel = "amazon.titan-embed-text-v2:0"

// vectorBucketName and vectorIndexName name the S3 Vectors store, and embedDimension is Titan v2's width.
const (
	vectorBucketName = "vault-vectors"
	vectorIndexName  = "files"
	embedDimension   = 1024
)

// filesKeyPrefix is the S3 key namespace for blobs, matching blob.keyPrefix in the backend.
const filesKeyPrefix = "files/"

// NewVaultStack defines the S3 bucket, DynamoDB index and API Lambda.
func NewVaultStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	webOrigin := stack.Node().TryGetContext(jsii.String("webOrigin"))
	origin, ok := webOrigin.(string)
	if !ok || origin == "" {
		origin = "http://localhost:3000"
	}

	// The frontend hosting is created first so its CloudFront origin can be allowed by
	// the API and the files bucket CORS below. localhost stays allowed for local dev.
	hosting := newFrontendHosting(stack)
	allowedOrigins := jsii.Strings(origin, hosting.URL())

	bucket := awss3.NewBucket(stack, jsii.String("Files"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		EnforceSSL:        jsii.Bool(true),
		Cors: &[]*awss3.CorsRule{{
			AllowedMethods: &[]awss3.HttpMethods{awss3.HttpMethods_PUT, awss3.HttpMethods_GET},
			AllowedOrigins: allowedOrigins,
			AllowedHeaders: jsii.Strings("*"),
		}},
		LifecycleRules: &[]*awss3.LifecycleRule{{
			Transitions: &[]*awss3.Transition{{
				StorageClass:    awss3.StorageClass_INTELLIGENT_TIERING(),
				TransitionAfter: awscdk.Duration_Days(jsii.Number(0)),
			}},
		}},
	})

	table := awsdynamodb.NewTableV2(stack, jsii.String("Index"), &awsdynamodb.TablePropsV2{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// callsTable holds the LLM call trace, one partition keyed by time, expired by TTL.
	callsTable := awsdynamodb.NewTableV2(stack, jsii.String("Calls"), &awsdynamodb.TablePropsV2{
		PartitionKey:        &awsdynamodb.Attribute{Name: jsii.String("pk"), Type: awsdynamodb.AttributeType_STRING},
		SortKey:             &awsdynamodb.Attribute{Name: jsii.String("sk"), Type: awsdynamodb.AttributeType_STRING},
		TimeToLiveAttribute: jsii.String("ttl"),
		RemovalPolicy:       awscdk.RemovalPolicy_DESTROY,
	})

	// The S3 Vectors bucket and index hold one embedding per file, keyed by file id, for semantic
	// search. CloudFormation supports these natively, so they are plain L1 resources.
	vectorBucket := awscdk.NewCfnResource(stack, jsii.String("VectorBucket"), &awscdk.CfnResourceProps{
		Type: jsii.String("AWS::S3Vectors::VectorBucket"),
		Properties: &map[string]interface{}{
			"VectorBucketName": jsii.String(vectorBucketName),
		},
	})
	vectorIndex := awscdk.NewCfnResource(stack, jsii.String("VectorIndex"), &awscdk.CfnResourceProps{
		Type: jsii.String("AWS::S3Vectors::Index"),
		Properties: &map[string]interface{}{
			"VectorBucketName": jsii.String(vectorBucketName),
			"IndexName":        jsii.String(vectorIndexName),
			"DataType":         jsii.String("float32"),
			"Dimension":        jsii.Number(embedDimension),
			"DistanceMetric":   jsii.String("cosine"),
		},
	})
	vectorIndex.AddDependency(vectorBucket)

	pool := awscognito.NewUserPool(stack, jsii.String("Users"), &awscognito.UserPoolProps{
		SelfSignUpEnabled: jsii.Bool(false),
		SignInAliases:     &awscognito.SignInAliases{Email: jsii.Bool(true)},
		PasswordPolicy: &awscognito.PasswordPolicy{
			MinLength:        jsii.Number(12),
			RequireLowercase: jsii.Bool(true),
			RequireUppercase: jsii.Bool(true),
			RequireDigits:    jsii.Bool(true),
			RequireSymbols:   jsii.Bool(true),
		},
		AccountRecovery: awscognito.AccountRecovery_EMAIL_ONLY,
		RemovalPolicy:   awscdk.RemovalPolicy_DESTROY,
	})

	client := pool.AddClient(jsii.String("ApiClient"), &awscognito.UserPoolClientOptions{
		GenerateSecret:      jsii.Bool(false),
		AuthFlows:           &awscognito.AuthFlow{UserPassword: jsii.Bool(true), UserSrp: jsii.Bool(true)},
		AccessTokenValidity: awscdk.Duration_Hours(jsii.Number(1)),
		IdTokenValidity:     awscdk.Duration_Hours(jsii.Number(1)),
	})

	fn := golambda.NewGoFunction(stack, jsii.String("Api"), &golambda.GoFunctionProps{
		Entry:   jsii.String("../backend/cmd/lambda"),
		Timeout: awscdk.Duration_Seconds(jsii.Number(30)),
		Environment: &map[string]*string{
			"VAULT_TABLE":          table.TableName(),
			"VAULT_BUCKET":         bucket.BucketName(),
			"VAULT_JWT_ISSUER":     pool.UserPoolProviderUrl(),
			"VAULT_JWT_CLIENT_ID":  client.UserPoolClientId(),
			"VAULT_BEDROCK_REGION": stack.Region(),
			"VAULT_EXTRACT_MODEL":  jsii.String(extractModel),
			"VAULT_RERANK_MODEL":   jsii.String(rerankModel),
			"VAULT_EMBED_MODEL":    jsii.String(embedModel),
			"VAULT_VECTOR_BUCKET":  jsii.String(vectorBucketName),
			"VAULT_VECTOR_INDEX":   jsii.String(vectorIndexName),
			"VAULT_CALLS_TABLE":    callsTable.TableName(),
		},
	})

	bucket.GrantReadWrite(fn, nil)
	bucket.GrantDelete(fn, nil)
	table.GrantReadWriteData(fn)
	callsTable.GrantReadWriteData(fn)

	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("bedrock:InvokeModel"),
		Resources: jsii.Strings(
			"arn:aws:bedrock:*::foundation-model/anthropic.*",
			"arn:aws:bedrock:*::foundation-model/amazon.titan-embed-text-v2:0",
			"arn:aws:bedrock:*:"+*stack.Account()+":inference-profile/*",
		),
	}))

	// The index and its parent bucket back semantic search: read on ask, write on drop, delete on remove.
	vectorArn := "arn:aws:s3vectors:" + *stack.Region() + ":" + *stack.Account() + ":bucket/" + vectorBucketName
	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings(
			"s3vectors:PutVectors",
			"s3vectors:QueryVectors",
			"s3vectors:DeleteVectors",
		),
		Resources: jsii.Strings(vectorArn, vectorArn+"/index/"+vectorIndexName),
	}))

	bucket.AddEventNotification(
		awss3.EventType_OBJECT_CREATED,
		awss3notifications.NewLambdaDestination(fn),
		&awss3.NotificationKeyFilter{Prefix: jsii.String(filesKeyPrefix)},
	)

	api := awsapigatewayv2.NewHttpApi(stack, jsii.String("HttpApi"), &awsapigatewayv2.HttpApiProps{
		CorsPreflight: &awsapigatewayv2.CorsPreflightOptions{
			AllowOrigins: allowedOrigins,
			AllowMethods: &[]awsapigatewayv2.CorsHttpMethod{
				awsapigatewayv2.CorsHttpMethod_GET,
				awsapigatewayv2.CorsHttpMethod_POST,
				awsapigatewayv2.CorsHttpMethod_PATCH,
				awsapigatewayv2.CorsHttpMethod_DELETE,
			},
			AllowHeaders: jsii.Strings("Content-Type", "Authorization"),
		},
	})

	integration := awsapigatewayv2integrations.NewHttpLambdaIntegration(jsii.String("ApiIntegration"), fn, nil)
	authorizer := awsapigatewayv2authorizers.NewHttpUserPoolAuthorizer(jsii.String("JwtAuthorizer"), pool, &awsapigatewayv2authorizers.HttpUserPoolAuthorizerProps{
		UserPoolClients: &[]awscognito.IUserPoolClient{client},
	})

	healthRoutes := api.AddRoutes(&awsapigatewayv2.AddRoutesOptions{
		Path:        jsii.String("/health"),
		Methods:     &[]awsapigatewayv2.HttpMethod{awsapigatewayv2.HttpMethod_GET},
		Integration: integration,
	})
	// Route the real verbs, not ANY: ANY would also match OPTIONS and send the CORS
	// preflight through the JWT authorizer (401), which fails the browser preflight.
	// Leaving OPTIONS unrouted lets the HTTP API answer preflight from CorsPreflight.
	api.AddRoutes(&awsapigatewayv2.AddRoutesOptions{
		Path: jsii.String("/{proxy+}"),
		Methods: &[]awsapigatewayv2.HttpMethod{
			awsapigatewayv2.HttpMethod_GET,
			awsapigatewayv2.HttpMethod_POST,
			awsapigatewayv2.HttpMethod_PATCH,
			awsapigatewayv2.HttpMethod_DELETE,
		},
		Integration: integration,
		Authorizer:  authorizer,
	})

	// Upload the built site and a config.json rendered from the stack outputs, so the SPA
	// reads its API and Cognito settings at runtime and never drifts from the backend.
	hosting.deploy(stack, api.Url(), pool.UserPoolId(), client.UserPoolClientId())

	awscdk.NewCfnOutput(stack, jsii.String("FrontendUrl"), &awscdk.CfnOutputProps{Value: jsii.String(hosting.URL())})
	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{Value: api.Url()})
	awscdk.NewCfnOutput(stack, jsii.String("BucketName"), &awscdk.CfnOutputProps{Value: bucket.BucketName()})
	awscdk.NewCfnOutput(stack, jsii.String("TableName"), &awscdk.CfnOutputProps{Value: table.TableName()})
	awscdk.NewCfnOutput(stack, jsii.String("UserPoolId"), &awscdk.CfnOutputProps{Value: pool.UserPoolId()})
	awscdk.NewCfnOutput(stack, jsii.String("UserPoolClientId"), &awscdk.CfnOutputProps{Value: client.UserPoolClientId()})

	suppressNag(stack, (*healthRoutes)[0])

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	NewVaultStack(app, "VaultStack", nil)
	NewVaultCICDStack(app, "VaultCICDStack", &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
			Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
		},
	})
	awscdk.Aspects_Of(app).Add(cdknag.NewAwsSolutionsChecks(&cdknag.NagPackProps{Verbose: jsii.Bool(true)}), nil)
	app.Synth(nil)
}
