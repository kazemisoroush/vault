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
	// kbCollectionName backs the Knowledge Base. It must match the AOSS name pattern (lowercase,
	// starts with a letter, 3-32 chars). kbCollectionGroup and kbVectorIndexName follow the same rule.
	kbCollectionName  = "vault-kb"
	kbCollectionGroup = "vault-kb-group"
	kbVectorIndexName = "vault-kb-index"

	// Bedrock's default field names for a Knowledge Base vector index. The index mapping and the
	// Knowledge Base field mapping must name the same three fields.
	kbVectorField   = "bedrock-knowledge-base-default-vector"
	kbTextField     = "AMAZON_BEDROCK_TEXT"
	kbMetadataField = "AMAZON_BEDROCK_METADATA"

	// titanEmbedDimensions is Amazon Titan Text Embeddings v2's output width.
	titanEmbedDimensions = 1024
)

// newKnowledgeBase stands up the managed retrieval foundation: an OpenSearch Serverless NextGen
// (scale-to-zero, no OCU floor) collection and its vector index, plus a Bedrock Knowledge Base over
// the files bucket that parses PDFs and image scans with Bedrock Data Automation so they become
// searchable. Hybrid search (vector + BM25) is requested at query time by the retrieval caller. It
// returns the Knowledge Base so the stack can output its id.
func newKnowledgeBase(stack awscdk.Stack, bucket awss3.Bucket) awsbedrock.CfnKnowledgeBase {
	region := stack.Region()

	// AOSS needs an encryption policy and a network policy in place before the collection exists.
	encryption := awsopensearchserverless.NewCfnSecurityPolicy(stack, jsii.String("KbEncryptionPolicy"), &awsopensearchserverless.CfnSecurityPolicyProps{
		Name: jsii.String("vault-kb-enc"),
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
		Name: jsii.String("vault-kb-net"),
		Type: jsii.String("network"),
		Policy: jsii.String(mustJSON([]map[string]any{{
			"Rules": []map[string]any{
				{"ResourceType": "collection", "Resource": []string{"collection/" + kbCollectionName}},
				{"ResourceType": "dashboard", "Resource": []string{"collection/" + kbCollectionName}},
			},
			"AllowFromPublic": true,
		}})),
	})

	// NextGen collection group: scale-to-zero with no standby replicas, so there is no OCU floor.
	group := awsopensearchserverless.NewCfnCollectionGroup(stack, jsii.String("KbCollectionGroup"), &awsopensearchserverless.CfnCollectionGroupProps{
		Name:            jsii.String(kbCollectionGroup),
		Generation:      jsii.String("NEXTGEN"),
		StandbyReplicas: jsii.String("DISABLED"),
	})

	collection := awsopensearchserverless.NewCfnCollection(stack, jsii.String("KbCollection"), &awsopensearchserverless.CfnCollectionProps{
		Name:                jsii.String(kbCollectionName),
		Type:                jsii.String("VECTORSEARCH"),
		CollectionGroupName: jsii.String(kbCollectionGroup),
	})
	collection.AddDependency(encryption)
	collection.AddDependency(network)
	collection.AddDependency(group)

	// The Knowledge Base's role, trusted by Bedrock: read the files bucket, invoke the embedding
	// model, and reach the collection's data plane.
	role := awsiam.NewRole(stack, jsii.String("KbRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("bedrock.amazonaws.com"), nil),
	})
	embedModelArn := jsii.String(fmt.Sprintf("arn:aws:bedrock:%s::foundation-model/amazon.titan-embed-text-v2:0", *region))
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

	// Data access policy: the KB role may read and write the collection and its index data plane.
	access := awsopensearchserverless.NewCfnAccessPolicy(stack, jsii.String("KbAccessPolicy"), &awsopensearchserverless.CfnAccessPolicyProps{
		Name: jsii.String("vault-kb-access"),
		Type: jsii.String("data"),
		Policy: jsii.String(mustJSON([]map[string]any{{
			"Rules": []map[string]any{
				{"ResourceType": "index", "Resource": []string{"index/" + kbCollectionName + "/*"}, "Permission": []string{"aoss:*"}},
				{"ResourceType": "collection", "Resource": []string{"collection/" + kbCollectionName}, "Permission": []string{"aoss:*"}},
			},
			"Principal": []string{*role.RoleArn()},
		}})),
	})

	// The vector index the Knowledge Base writes to. Field names must match the KB field mapping.
	index := awsopensearchserverless.NewCfnIndex(stack, jsii.String("KbVectorIndex"), &awsopensearchserverless.CfnIndexProps{
		CollectionEndpoint: collection.AttrCollectionEndpoint(),
		IndexName:          jsii.String(kbVectorIndexName),
		Settings: map[string]any{
			"index": map[string]any{"knn": true},
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				kbVectorField: map[string]any{
					"type":      "knn_vector",
					"dimension": titanEmbedDimensions,
					"method":    map[string]any{"name": "hnsw", "engine": "faiss", "spaceType": "l2"},
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
