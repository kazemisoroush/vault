package main

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbedrock"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsopensearchserverless"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
)

const (
	// kbName is the shared prefix for the Knowledge Base's OpenSearch resources. The collection name
	// must match the AOSS pattern (lowercase, starts with a letter, 3-32 chars); the rest derive from
	// it so the prefix lives in one place.
	kbName            = "vault-kb"
	kbCollectionName  = kbName
	kbCollectionGroup = kbName + "-group"
	kbVectorIndexName = kbName + "-index"

	kbEncryptionPolicyName = kbName + "-enc"
	kbNetworkPolicyName    = kbName + "-net"
	kbAccessPolicyName     = kbName + "-access"

	// Bedrock's default field names for a Knowledge Base vector index. The index mapping and the
	// Knowledge Base field mapping must name the same three fields.
	kbVectorField   = "bedrock-knowledge-base-default-vector"
	kbTextField     = "AMAZON_BEDROCK_TEXT"
	kbMetadataField = "AMAZON_BEDROCK_METADATA"

	// cdkQualifier is the CDK bootstrap qualifier this repo deploys with, so the CloudFormation
	// execution role that creates the vector index can be granted data-plane access below. The repo
	// uses the default qualifier (see cicd_test.go, which asserts cdk-hnb659fds-* roles).
	cdkQualifier = "hnb659fds"
)

// newKnowledgeBase stands up the managed retrieval foundation: an OpenSearch Serverless NextGen
// (scale-to-zero, no OCU floor) collection and its vector index, plus a Bedrock Knowledge Base over
// the files bucket that parses PDFs and image scans with Bedrock Data Automation so they become
// searchable. It provisions storage and ingestion only; querying the Knowledge Base, where hybrid
// vector plus BM25 search is applied, is a later change. It returns the Knowledge Base so the stack
// can output its id.
func newKnowledgeBase(stack awscdk.Stack, bucket awss3.Bucket) awsbedrock.CfnKnowledgeBase {
	region := stack.Region()
	account := stack.Account()

	// AOSS needs an encryption policy and a network policy in place before the collection exists.
	encryption := awsopensearchserverless.NewCfnSecurityPolicy(stack, jsii.String("KbEncryptionPolicy"), &awsopensearchserverless.CfnSecurityPolicyProps{
		Name: jsii.String(kbEncryptionPolicyName),
		Type: jsii.String("encryption"),
		Policy: jsii.String(mustJSON(map[string]any{
			"Rules": []map[string]any{{
				"ResourceType": "collection",
				"Resource":     []string{"collection/" + kbCollectionName},
			}},
			"AWSOwnedKey": true,
		})),
	})
	network := awsopensearchserverless.NewCfnSecurityPolicy(stack, jsii.String("KbNetworkPolicy"), &awsopensearchserverless.CfnSecurityPolicyProps{
		Name: jsii.String(kbNetworkPolicyName),
		Type: jsii.String("network"),
		Policy: jsii.String(mustJSON([]map[string]any{{
			"Rules": []map[string]any{
				{"ResourceType": "collection", "Resource": []string{"collection/" + kbCollectionName}},
				{"ResourceType": "dashboard", "Resource": []string{"collection/" + kbCollectionName}},
			},
			"AllowFromPublic": true,
		}})),
	})

	// NextGen collection group: the scale-to-zero generation, so there is no OCU floor. NextGen
	// requires standby replicas ENABLED (the service rejects DISABLED for this generation).
	group := awsopensearchserverless.NewCfnCollectionGroup(stack, jsii.String("KbCollectionGroup"), &awsopensearchserverless.CfnCollectionGroupProps{
		Name:            jsii.String(kbCollectionGroup),
		Generation:      jsii.String("NEXTGEN"),
		StandbyReplicas: jsii.String("ENABLED"),
	})

	collection := awsopensearchserverless.NewCfnCollection(stack, jsii.String("KbCollection"), &awsopensearchserverless.CfnCollectionProps{
		Name:                jsii.String(kbCollectionName),
		Type:                jsii.String("VECTORSEARCH"),
		CollectionGroupName: jsii.String(kbCollectionGroup),
	})
	collection.AddDependency(encryption)
	collection.AddDependency(network)
	collection.AddDependency(group)

	// The Knowledge Base's role, trusted by Bedrock only for this account's knowledge bases (closing
	// the cross-account confused-deputy path), reading the files bucket, invoking the embedding model,
	// running Bedrock Data Automation to parse scans, and reaching the collection's data plane.
	role := awsiam.NewRole(stack, jsii.String("KbRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewPrincipalWithConditions(
			awsiam.NewServicePrincipal(jsii.String("bedrock.amazonaws.com"), nil),
			&map[string]interface{}{
				"StringEquals": map[string]interface{}{"aws:SourceAccount": account},
				"ArnLike":      map[string]interface{}{"aws:SourceArn": jsii.String(fmt.Sprintf("arn:aws:bedrock:%s:%s:knowledge-base/*", *region, *account))},
			},
		),
	})
	embedModelArn := jsii.String(fmt.Sprintf("arn:aws:bedrock:%s::foundation-model/%s", *region, embedModel))
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("bedrock:InvokeModel"),
		Resources: &[]*string{embedModelArn},
	}))
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("s3:GetObject", "s3:ListBucket"),
		Resources: &[]*string{bucket.BucketArn(), jsii.String(*bucket.BucketArn() + "/*")},
	}))
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("aoss:APIAccessAll"),
		Resources: &[]*string{collection.AttrArn()},
	}))
	// Bedrock Data Automation, used by the data source to parse PDFs and image scans.
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("bedrock:InvokeDataAutomationAsync"),
		Resources: &[]*string{
			jsii.String(fmt.Sprintf("arn:aws:bedrock:%s:aws:data-automation-project/public-rag-default", *region)),
			jsii.String(fmt.Sprintf("arn:aws:bedrock:%s::data-automation-profile/*", *region)),
		},
	}))
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("bedrock:GetDataAutomationStatus"),
		Resources: &[]*string{jsii.String(fmt.Sprintf("arn:aws:bedrock:%s::data-automation-invocation/*", *region))},
	}))

	// Data access policy: the KB role reads and writes documents, and the CloudFormation execution
	// role that creates the index needs the index management verbs, or the deploy fails. Scoped to
	// the specific data-plane actions each needs rather than aoss:*.
	cfnExecRoleArn := fmt.Sprintf("arn:aws:iam::%s:role/cdk-%s-cfn-exec-role-%s-%s", *account, cdkQualifier, *account, *region)
	access := awsopensearchserverless.NewCfnAccessPolicy(stack, jsii.String("KbAccessPolicy"), &awsopensearchserverless.CfnAccessPolicyProps{
		Name: jsii.String(kbAccessPolicyName),
		Type: jsii.String("data"),
		Policy: jsii.String(mustJSON([]map[string]any{{
			"Rules": []map[string]any{
				{"ResourceType": "index", "Resource": []string{"index/" + kbCollectionName + "/*"}, "Permission": []string{
					"aoss:CreateIndex", "aoss:DeleteIndex", "aoss:UpdateIndex", "aoss:DescribeIndex", "aoss:ReadDocument", "aoss:WriteDocument",
				}},
				{"ResourceType": "collection", "Resource": []string{"collection/" + kbCollectionName}, "Permission": []string{
					"aoss:CreateCollectionItems", "aoss:UpdateCollectionItems", "aoss:DescribeCollectionItems",
				}},
			},
			"Principal": []string{*role.RoleArn(), cfnExecRoleArn},
		}})),
	})

	// The vector index the Knowledge Base writes to. Field names must match the KB field mapping.
	// A NextGen collection auto-selects the vector engine and method, so the field carries only its
	// type, dimension, and space type; specifying an explicit engine/method (as a Classic collection
	// would) is rejected with "Field parameter 'engine' is not supported".
	index := awsopensearchserverless.NewCfnIndex(stack, jsii.String("KbVectorIndex"), &awsopensearchserverless.CfnIndexProps{
		CollectionEndpoint: collection.AttrCollectionEndpoint(),
		IndexName:          jsii.String(kbVectorIndexName),
		Settings: map[string]any{
			"index": map[string]any{"knn": true},
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				kbVectorField: map[string]any{
					"type":       "knn_vector",
					"dimension":  embedDimension,
					"space_type": "l2",
				},
				kbTextField:     map[string]any{"type": "text"},
				kbMetadataField: map[string]any{"type": "text", "index": false},
			},
		},
	})
	index.AddDependency(collection)
	index.AddDependency(access)

	kb := awsbedrock.NewCfnKnowledgeBase(stack, jsii.String("KnowledgeBase"), &awsbedrock.CfnKnowledgeBaseProps{
		Name:    jsii.String("VaultKnowledgeBase"),
		RoleArn: role.RoleArn(),
		KnowledgeBaseConfiguration: &awsbedrock.CfnKnowledgeBase_KnowledgeBaseConfigurationProperty{
			Type: jsii.String("VECTOR"),
			VectorKnowledgeBaseConfiguration: &awsbedrock.CfnKnowledgeBase_VectorKnowledgeBaseConfigurationProperty{
				EmbeddingModelArn: embedModelArn,
			},
		},
		StorageConfiguration: &awsbedrock.CfnKnowledgeBase_StorageConfigurationProperty{
			Type: jsii.String("OPENSEARCH_SERVERLESS"),
			OpensearchServerlessConfiguration: &awsbedrock.CfnKnowledgeBase_OpenSearchServerlessConfigurationProperty{
				CollectionArn:   collection.AttrArn(),
				VectorIndexName: jsii.String(kbVectorIndexName),
				FieldMapping: &awsbedrock.CfnKnowledgeBase_OpenSearchServerlessFieldMappingProperty{
					VectorField:   jsii.String(kbVectorField),
					TextField:     jsii.String(kbTextField),
					MetadataField: jsii.String(kbMetadataField),
				},
			},
		},
	})
	kb.AddDependency(index)

	// The S3 data source, parsed by Bedrock Data Automation so PDFs and image scans yield text and
	// entities (the passport is an image; without this it would carry no searchable text).
	awsbedrock.NewCfnDataSource(stack, jsii.String("KbDataSource"), &awsbedrock.CfnDataSourceProps{
		KnowledgeBaseId: kb.AttrKnowledgeBaseId(),
		Name:            jsii.String("VaultFiles"),
		DataSourceConfiguration: &awsbedrock.CfnDataSource_DataSourceConfigurationProperty{
			Type: jsii.String("S3"),
			S3Configuration: &awsbedrock.CfnDataSource_S3DataSourceConfigurationProperty{
				BucketArn: bucket.BucketArn(),
			},
		},
		VectorIngestionConfiguration: &awsbedrock.CfnDataSource_VectorIngestionConfigurationProperty{
			ParsingConfiguration: &awsbedrock.CfnDataSource_ParsingConfigurationProperty{
				ParsingStrategy: jsii.String("BEDROCK_DATA_AUTOMATION"),
				BedrockDataAutomationConfiguration: &awsbedrock.CfnDataSource_BedrockDataAutomationConfigurationProperty{
					ParsingModality: jsii.String("MULTIMODAL"),
				},
			},
		},
	})

	awscdk.NewCfnOutput(stack, jsii.String("KnowledgeBaseId"), &awscdk.CfnOutputProps{Value: kb.AttrKnowledgeBaseId()})
	awscdk.NewCfnOutput(stack, jsii.String("KbCollectionArn"), &awscdk.CfnOutputProps{Value: collection.AttrArn()})

	return kb
}

// mustJSON renders an AOSS policy document to the compact JSON string the L1 property expects.
func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
