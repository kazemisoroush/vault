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
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3notifications"
	golambda "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdklabs/cdk-nag-go/cdknag/v2"
)

// claudeModel is the Bedrock Claude inference profile the ask agent and the check pipeline call.
const claudeModel = "us.anthropic.claude-haiku-4-5-20251001-v1:0"

// rerankModel names the model the backend loads for the agent and the check: the same Claude model.
const rerankModel = claudeModel

// embedModel is the Knowledge Base's embedding model and embedDimension is its vector width.
const embedModel = "amazon.titan-embed-text-v2:0"
const embedDimension = 1024

// stagingKeyPrefix is where fresh uploads land before ingest hashes them, matching blob.stagingPrefix.
const stagingKeyPrefix = "uploads/"

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
		LifecycleRules: &[]*awss3.LifecycleRule{
			{
				Transitions: &[]*awss3.Transition{{
					StorageClass:    awss3.StorageClass_INTELLIGENT_TIERING(),
					TransitionAfter: awscdk.Duration_Days(jsii.Number(0)),
				}},
			},
			{
				// A failed ingest can leave a staging object behind, so expire the staging prefix.
				Prefix:     jsii.String(stagingKeyPrefix),
				Expiration: awscdk.Duration_Days(jsii.Number(1)),
			},
		},
	})

	table := awsdynamodb.NewTableV2(stack, jsii.String("Index"), &awsdynamodb.TablePropsV2{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		// The status index lets the Knowledge Base syncer query the landed files without scanning
		// the whole table. It projects every attribute so the syncer advances a record with one write.
		GlobalSecondaryIndexes: &[]*awsdynamodb.GlobalSecondaryIndexPropsV2{
			{
				IndexName:      jsii.String("status-index"),
				PartitionKey:   &awsdynamodb.Attribute{Name: jsii.String("status"), Type: awsdynamodb.AttributeType_STRING},
				ProjectionType: awsdynamodb.ProjectionType_ALL,
			},
		},
	})

	// callsTable holds the LLM call trace, one partition keyed by time, expired by TTL.
	callsTable := awsdynamodb.NewTableV2(stack, jsii.String("Calls"), &awsdynamodb.TablePropsV2{
		PartitionKey:        &awsdynamodb.Attribute{Name: jsii.String("pk"), Type: awsdynamodb.AttributeType_STRING},
		SortKey:             &awsdynamodb.Attribute{Name: jsii.String("sk"), Type: awsdynamodb.AttributeType_STRING},
		TimeToLiveAttribute: jsii.String("ttl"),
		RemovalPolicy:       awscdk.RemovalPolicy_DESTROY,
	})

	// checksTable holds check runs: pasted text, claims, and verdicts.
	checksTable := awsdynamodb.NewTableV2(stack, jsii.String("Checks"), &awsdynamodb.TablePropsV2{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// The managed Bedrock Knowledge Base over the files bucket now serves both retrieval (hybrid
	// search) and ingestion (the scheduled syncer runs its jobs). It is the only retrieval infra.
	knowledge := newKnowledgeBase(stack, bucket)

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
		Entry: jsii.String("../backend/cmd/lambda"),
		// The ask agent runs several model calls in one request as it queries and answers, so it
		// needs more than the default headroom.
		Timeout: awscdk.Duration_Seconds(jsii.Number(120)),
		// Ingest reads a whole object into memory, and unpacking a dropped zip holds the archive
		// plus an inner file at once, so the default 128 MB is not enough.
		MemorySize: jsii.Number(1024),
		Environment: &map[string]*string{
			"TABLE":                         table.TableName(),
			"BUCKET":                        bucket.BucketName(),
			"JWT_ISSUER":                    pool.UserPoolProviderUrl(),
			"JWT_CLIENT_ID":                 client.UserPoolClientId(),
			"BEDROCK_REGION":                stack.Region(),
			"RERANK_MODEL":                  jsii.String(rerankModel),
			"KNOWLEDGE_BASE_ID":             knowledge.id,
			"KNOWLEDGE_BASE_DATA_SOURCE_ID": knowledge.dataSourceID,
			"CALLS_TABLE":                   callsTable.TableName(),
			"CHECKS_TABLE":                  checksTable.TableName(),
		},
	})

	bucket.GrantReadWrite(fn, nil)
	bucket.GrantDelete(fn, nil)
	table.GrantReadWriteData(fn)
	callsTable.GrantReadWriteData(fn)
	checksTable.GrantReadWriteData(fn)

	// The check pipeline runs as an async self-invocation (the API reply returns immediately and
	// the pipeline gets the full timeout). The function's own name is unknowable here without a
	// circular reference, so the grant is scoped by the stack's function name prefix.
	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("lambda:InvokeFunction"),
		Resources: jsii.Strings("arn:aws:lambda:" + *stack.Region() + ":" + *stack.Account() + ":function:" + *stack.StackName() + "-Api*"),
	}))

	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("bedrock:InvokeModel"),
		Resources: jsii.Strings(
			"arn:aws:bedrock:*::foundation-model/anthropic.*",
			"arn:aws:bedrock:*:"+*stack.Account()+":inference-profile/*",
		),
	}))

	// The Knowledge Base backs both retrieval (the agent and check query it by hybrid search) and
	// ingestion (the scheduled syncer starts and polls ingestion jobs on the data source).
	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings(
			"bedrock:Retrieve",
			"bedrock:StartIngestionJob",
			"bedrock:GetIngestionJob",
		),
		Resources: jsii.Strings("arn:aws:bedrock:" + *stack.Region() + ":" + *stack.Account() + ":knowledge-base/" + *knowledge.id),
	}))

	// Ingest runs as an async invocation from the S3 event. A transient failure, such as the
	// metadata sidecar write, returns an error on purpose, so let Lambda redrive the event a few
	// times over a bounded window rather than losing the file to a terminal failure.
	fn.ConfigureAsyncInvoke(&awslambda.EventInvokeConfigOptions{
		RetryAttempts: jsii.Number(2),
		MaxEventAge:   awscdk.Duration_Minutes(jsii.Number(15)),
	})

	// Watch only the staging prefix so the content-addressed copy under files/ does not re-trigger ingest.
	bucket.AddEventNotification(
		awss3.EventType_OBJECT_CREATED,
		awss3notifications.NewLambdaDestination(fn),
		&awss3.NotificationKeyFilter{Prefix: jsii.String(stagingKeyPrefix)},
	)

	// A schedule drives the Knowledge Base sync: the syncer advances landed files to ingested a
	// short while after they land, running at most one ingestion job at a time.
	awsevents.NewRule(stack, jsii.String("KbSyncSchedule"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(2))),
		Targets:  &[]awsevents.IRuleTarget{awseventstargets.NewLambdaFunction(fn, nil)},
	})

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
