package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbedrock"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsopensearchservice"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/customresources"
	"github.com/aws/jsii-runtime-go"
)

const (
	// kbDomainName names the OpenSearch managed domain (lowercase, 3-28 chars, starts with a letter).
	kbDomainName      = "vault-kb"
	kbVectorIndexName = "vault-kb-index"

	// Bedrock's default field names for a Knowledge Base vector index. The index the Lambda creates
	// and the Knowledge Base field mapping must name the same three fields.
	kbVectorField   = "bedrock-knowledge-base-default-vector"
	kbTextField     = "AMAZON_BEDROCK_TEXT"
	kbMetadataField = "AMAZON_BEDROCK_METADATA"
)

// indexInitCode is the inline handler for the custom resource that creates the vector index. A
// managed OpenSearch domain has no native CloudFormation index resource, so on create it signs a
// PUT to the domain with the k-NN mapping (FAISS/HNSW, the engine Bedrock requires), and on delete
// it removes the index. The Provider framework wraps the CloudFormation response protocol.
const indexInitCode = `
import json, urllib3, botocore.session
from botocore.auth import SigV4Auth
from botocore.awsrequest import AWSRequest

http = urllib3.PoolManager()

def _signed(method, url, body, region):
    creds = botocore.session.Session().get_credentials().get_frozen_credentials()
    req = AWSRequest(method=method, url=url, data=body, headers={"Content-Type": "application/json"})
    SigV4Auth(creds, "es", region).add_auth(req)
    return http.request(method, url, body=body, headers=dict(req.headers))

def handler(event, context):
    p = event["ResourceProperties"]
    url = p["Endpoint"].rstrip("/") + "/" + p["IndexName"]
    pid = "index-" + p["IndexName"]
    if event["RequestType"] == "Delete":
        _signed("DELETE", url, None, p["Region"])
        return {"PhysicalResourceId": pid}
    mapping = {
        "settings": {"index": {"knn": True}},
        "mappings": {"properties": {
            p["VectorField"]: {"type": "knn_vector", "dimension": int(p["Dimension"]),
                "method": {"name": "hnsw", "engine": "faiss", "space_type": "l2"}},
            p["TextField"]: {"type": "text"},
            p["MetadataField"]: {"type": "text", "index": False},
        }},
    }
    r = _signed("PUT", url, json.dumps(mapping), p["Region"])
    if r.status not in (200, 201) and b"resource_already_exists_exception" not in r.data:
        raise Exception("create index failed: %d %s" % (r.status, r.data[:500]))
    return {"PhysicalResourceId": pid}
`

// newKnowledgeBase stands up the managed retrieval foundation: an OpenSearch managed domain, its
// vector index (created by a custom-resource Lambda, since a managed domain has no native
// CloudFormation index resource), and a Bedrock Knowledge Base over the files bucket that parses
// PDFs and image scans with Bedrock Data Automation. The Knowledge Base serves both the hybrid
// vector plus BM25 search on query and the ingestion the syncer drives.
func newKnowledgeBase(stack awscdk.Stack, bucket awss3.Bucket) knowledgeBase {
	region := stack.Region()
	account := stack.Account()

	// Bedrock Data Automation stores its extracted multimodal data here. Bedrock requires a bucket
	// root URI with no sub-folder, so this is a dedicated bucket kept apart from the user's files.
	supplemental := awss3.NewBucket(stack, jsii.String("KbSupplemental"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		EnforceSSL:        jsii.Bool(true),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
	})

	// Managed OpenSearch domain: one small node, encrypted at rest and in transit, HTTPS only. The
	// access policy allows this account's principals (identity policies below scope who actually
	// reaches it); fine-grained access control is optional and not used.
	domainArn := fmt.Sprintf("arn:aws:es:%s:%s:domain/%s", *region, *account, kbDomainName)
	domain := awsopensearchservice.NewDomain(stack, jsii.String("KbDomain"), &awsopensearchservice.DomainProps{
		DomainName: jsii.String(kbDomainName),
		Version:    awsopensearchservice.EngineVersion_OPENSEARCH_2_13(),
		Capacity: &awsopensearchservice.CapacityConfig{
			DataNodes:            jsii.Number(1),
			DataNodeInstanceType: jsii.String("t3.small.search"),
		},
		Ebs: &awsopensearchservice.EbsOptions{
			Enabled:    jsii.Bool(true),
			VolumeSize: jsii.Number(10),
			VolumeType: awsec2.EbsDeviceVolumeType_GP3,
		},
		EncryptionAtRest:     &awsopensearchservice.EncryptionAtRestOptions{Enabled: jsii.Bool(true)},
		NodeToNodeEncryption: jsii.Bool(true),
		EnforceHttps:         jsii.Bool(true),
		AccessPolicies: &[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect:     awsiam.Effect_ALLOW,
				Principals: &[]awsiam.IPrincipal{awsiam.NewAccountRootPrincipal()},
				Actions:    jsii.Strings("es:ESHttp*"),
				Resources:  jsii.Strings(domainArn + "/*"),
			}),
		},
	})
	domain.ApplyRemovalPolicy(awscdk.RemovalPolicy_DESTROY)

	// The Knowledge Base's role, trusted by Bedrock only for this account's knowledge bases (closing
	// the cross-account confused-deputy path): read the files bucket, invoke the embedding model, run
	// Bedrock Data Automation to parse scans, and reach the domain over its HTTP API.
	embedModelArn := jsii.String(fmt.Sprintf("arn:aws:bedrock:%s::foundation-model/%s", *region, embedModel))
	role := awsiam.NewRole(stack, jsii.String("KbRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewPrincipalWithConditions(
			awsiam.NewServicePrincipal(jsii.String("bedrock.amazonaws.com"), nil),
			&map[string]interface{}{
				"StringEquals": map[string]interface{}{"aws:SourceAccount": account},
				"ArnLike":      map[string]interface{}{"aws:SourceArn": jsii.String(fmt.Sprintf("arn:aws:bedrock:%s:%s:knowledge-base/*", *region, *account))},
			},
		),
	})
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("bedrock:InvokeModel"),
		Resources: &[]*string{embedModelArn},
	}))
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("s3:GetObject", "s3:ListBucket"),
		Resources: &[]*string{bucket.BucketArn(), jsii.String(*bucket.BucketArn() + "/*")},
	}))
	// Read and write access to the supplemental bucket for Bedrock Data Automation's output.
	supplemental.GrantReadWrite(role, nil)
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
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("es:DescribeDomain"),
		Resources: &[]*string{domain.DomainArn()},
	}))
	// es:ESHttp* on the domain for the Knowledge Base to read and write documents.
	domain.GrantReadWrite(role)

	// The custom-resource Lambda that creates the vector index, and its access to the domain.
	indexFn := awslambda.NewFunction(stack, jsii.String("KbIndexInit"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PYTHON_3_12(),
		Handler: jsii.String("index.handler"),
		Timeout: awscdk.Duration_Minutes(jsii.Number(5)),
		Code:    awslambda.Code_FromInline(jsii.String(indexInitCode)),
	})
	domain.GrantReadWrite(indexFn)

	provider := customresources.NewProvider(stack, jsii.String("KbIndexProvider"), &customresources.ProviderProps{
		OnEventHandler: indexFn,
	})
	endpointURL := "https://" + *domain.DomainEndpoint()
	index := awscdk.NewCustomResource(stack, jsii.String("KbVectorIndex"), &awscdk.CustomResourceProps{
		ServiceToken: provider.ServiceToken(),
		Properties: &map[string]interface{}{
			"Endpoint":      jsii.String(endpointURL),
			"IndexName":     jsii.String(kbVectorIndexName),
			"Region":        region,
			"VectorField":   jsii.String(kbVectorField),
			"TextField":     jsii.String(kbTextField),
			"MetadataField": jsii.String(kbMetadataField),
			"Dimension":     jsii.Number(embedDimension),
		},
	})
	index.Node().AddDependency(domain)

	kb := awsbedrock.NewCfnKnowledgeBase(stack, jsii.String("KnowledgeBase"), &awsbedrock.CfnKnowledgeBaseProps{
		Name:    jsii.String("VaultKnowledgeBase"),
		RoleArn: role.RoleArn(),
		KnowledgeBaseConfiguration: &awsbedrock.CfnKnowledgeBase_KnowledgeBaseConfigurationProperty{
			Type: jsii.String("VECTOR"),
			VectorKnowledgeBaseConfiguration: &awsbedrock.CfnKnowledgeBase_VectorKnowledgeBaseConfigurationProperty{
				EmbeddingModelArn: embedModelArn,
				// Bedrock Data Automation multimodal parsing extracts images and media from scans, so
				// the Knowledge Base needs an S3 location to store that supplemental data.
				SupplementalDataStorageConfiguration: &awsbedrock.CfnKnowledgeBase_SupplementalDataStorageConfigurationProperty{
					SupplementalDataStorageLocations: &[]interface{}{
						&awsbedrock.CfnKnowledgeBase_SupplementalDataStorageLocationProperty{
							SupplementalDataStorageLocationType: jsii.String("S3"),
							S3Location: &awsbedrock.CfnKnowledgeBase_S3LocationProperty{
								Uri: jsii.String("s3://" + *supplemental.BucketName()),
							},
						},
					},
				},
			},
		},
		StorageConfiguration: &awsbedrock.CfnKnowledgeBase_StorageConfigurationProperty{
			Type: jsii.String("OPENSEARCH_MANAGED_CLUSTER"),
			OpensearchManagedClusterConfiguration: &awsbedrock.CfnKnowledgeBase_OpenSearchManagedClusterConfigurationProperty{
				DomainArn:       domain.DomainArn(),
				DomainEndpoint:  jsii.String(endpointURL),
				VectorIndexName: jsii.String(kbVectorIndexName),
				FieldMapping: &awsbedrock.CfnKnowledgeBase_OpenSearchManagedClusterFieldMappingProperty{
					VectorField:   jsii.String(kbVectorField),
					TextField:     jsii.String(kbTextField),
					MetadataField: jsii.String(kbMetadataField),
				},
			},
		},
	})
	kb.Node().AddDependency(index)

	// The S3 data source, parsed by Bedrock Data Automation so PDFs and image scans yield text and
	// entities (the passport is an image; without this it would carry no searchable text). Only the
	// files/ prefix is ingested, so staged uploads and metadata elsewhere never reach the index.
	dataSource := awsbedrock.NewCfnDataSource(stack, jsii.String("KbDataSource"), &awsbedrock.CfnDataSourceProps{
		KnowledgeBaseId: kb.AttrKnowledgeBaseId(),
		Name:            jsii.String("VaultFiles"),
		DataSourceConfiguration: &awsbedrock.CfnDataSource_DataSourceConfigurationProperty{
			Type: jsii.String("S3"),
			S3Configuration: &awsbedrock.CfnDataSource_S3DataSourceConfigurationProperty{
				BucketArn:         bucket.BucketArn(),
				InclusionPrefixes: jsii.Strings("files/"),
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
	awscdk.NewCfnOutput(stack, jsii.String("KbDomainEndpoint"), &awscdk.CfnOutputProps{Value: domain.DomainEndpoint()})

	return knowledgeBase{id: kb.AttrKnowledgeBaseId(), dataSourceID: dataSource.AttrDataSourceId()}
}

// knowledgeBase carries the ids the Lambda needs to query and sync the managed Knowledge Base.
type knowledgeBase struct {
	id           *string
	dataSourceID *string
}
