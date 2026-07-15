package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
)

// TestKnowledgeBaseStandsUpManagedHybridFoundation checks the managed retrieval foundation: an
// encrypted OpenSearch managed domain, a custom-resource that creates the vector index, a Bedrock
// Knowledge Base on the managed cluster, and a data source that parses scans with Bedrock Data
// Automation.
func TestKnowledgeBaseStandsUpManagedHybridFoundation(t *testing.T) {
	// Arrange
	defer jsii.Close()
	app := awscdk.NewApp(nil)
	env := &awscdk.Environment{Account: jsii.String("111111111111"), Region: jsii.String("us-east-1")}
	stack := awscdk.NewStack(app, jsii.String("TestKb"), &awscdk.StackProps{Env: env})
	bucket := awss3.NewBucket(stack, jsii.String("Files"), nil)

	// Act
	newKnowledgeBase(stack, bucket)
	template := assertions.Template_FromStack(stack, nil)

	// Assert: an encrypted, HTTPS-only OpenSearch domain on a supported version.
	template.HasResourceProperties(jsii.String("AWS::OpenSearchService::Domain"), map[string]any{
		"EngineVersion":               "OpenSearch_2.13",
		"DomainEndpointOptions":       assertions.Match_ObjectLike(&map[string]any{"EnforceHTTPS": true}),
		"EncryptionAtRestOptions":     assertions.Match_ObjectLike(&map[string]any{"Enabled": true}),
		"NodeToNodeEncryptionOptions": assertions.Match_ObjectLike(&map[string]any{"Enabled": true}),
	})

	// Assert: a custom resource creates the vector index (managed domains have no native index resource).
	template.HasResourceProperties(jsii.String("AWS::CloudFormation::CustomResource"), map[string]any{
		"IndexName": kbVectorIndexName,
	})

	// Assert: the Knowledge Base is backed by the managed cluster.
	template.HasResourceProperties(jsii.String("AWS::Bedrock::KnowledgeBase"), map[string]any{
		"StorageConfiguration": assertions.Match_ObjectLike(&map[string]any{"Type": "OPENSEARCH_MANAGED_CLUSTER"}),
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
