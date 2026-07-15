package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
)

// TestKnowledgeBaseStandsUpNextGenHybridFoundation checks the managed retrieval foundation: a
// scale-to-zero NextGen collection, a vector index, a Bedrock Knowledge Base on OpenSearch
// Serverless, and a data source that parses scans with Bedrock Data Automation.
func TestKnowledgeBaseStandsUpNextGenHybridFoundation(t *testing.T) {
	// Arrange
	defer jsii.Close()
	app := awscdk.NewApp(nil)
	env := &awscdk.Environment{Account: jsii.String("111111111111"), Region: jsii.String("us-east-1")}
	stack := awscdk.NewStack(app, jsii.String("TestKb"), &awscdk.StackProps{Env: env})
	bucket := awss3.NewBucket(stack, jsii.String("Files"), nil)

	// Act
	newKnowledgeBase(stack, bucket)
	template := assertions.Template_FromStack(stack, nil)

	// Assert: NextGen scale-to-zero (no OCU floor), one collection, and one vector index.
	template.HasResourceProperties(jsii.String("AWS::OpenSearchServerless::CollectionGroup"), map[string]any{
		"Generation":      "NEXTGEN",
		"StandbyReplicas": "DISABLED",
	})
	template.ResourceCountIs(jsii.String("AWS::OpenSearchServerless::Collection"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::OpenSearchServerless::Index"), jsii.Number(1))

	// Assert: the Knowledge Base is backed by OpenSearch Serverless.
	template.HasResourceProperties(jsii.String("AWS::Bedrock::KnowledgeBase"), map[string]any{
		"StorageConfiguration": assertions.Match_ObjectLike(&map[string]any{"Type": "OPENSEARCH_SERVERLESS"}),
	})

	// Assert: the data source parses scans and PDFs with Bedrock Data Automation.
	template.HasResourceProperties(jsii.String("AWS::Bedrock::DataSource"), map[string]any{
		"VectorIngestionConfiguration": assertions.Match_ObjectLike(&map[string]any{
			"ParsingConfiguration": assertions.Match_ObjectLike(&map[string]any{
				"ParsingStrategy": "BEDROCK_DATA_AUTOMATION",
			}),
		}),
	})
}
