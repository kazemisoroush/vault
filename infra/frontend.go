package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
	"github.com/aws/jsii-runtime-go"
)

// frontendHosting is the private static-site bucket and the CloudFront distribution in front of it.
type frontendHosting struct {
	bucket       awss3.Bucket
	distribution awscloudfront.Distribution
}

// newFrontendHosting creates a private S3 bucket served as an SPA through CloudFront over HTTPS.
func newFrontendHosting(stack awscdk.Stack) frontendHosting {
	bucket := awss3.NewBucket(stack, jsii.String("Web"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		EnforceSSL:        jsii.Bool(true),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
	})

	distribution := awscloudfront.NewDistribution(stack, jsii.String("WebCdn"), &awscloudfront.DistributionProps{
		DefaultRootObject: jsii.String("index.html"),
		DefaultBehavior: &awscloudfront.BehaviorOptions{
			Origin:               awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(bucket, nil),
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		},
		PriceClass: awscloudfront.PriceClass_PRICE_CLASS_100,
		// SPA routing: client-side routes 404/403 at the edge return index.html so the app can render them.
		ErrorResponses: &[]*awscloudfront.ErrorResponse{
			{HttpStatus: jsii.Number(403), ResponseHttpStatus: jsii.Number(200), ResponsePagePath: jsii.String("/index.html")},
			{HttpStatus: jsii.Number(404), ResponseHttpStatus: jsii.Number(200), ResponsePagePath: jsii.String("/index.html")},
		},
	})

	return frontendHosting{bucket: bucket, distribution: distribution}
}

// deploy uploads the built static site plus a config.json rendered from the stack outputs,
// then invalidates the distribution so the new version is served immediately.
func (f frontendHosting) deploy(stack awscdk.Stack, config map[string]any) {
	awss3deployment.NewBucketDeployment(stack, jsii.String("WebDeploy"), &awss3deployment.BucketDeploymentProps{
		DestinationBucket: f.bucket,
		Distribution:      f.distribution,
		DistributionPaths: jsii.Strings("/*"),
		Sources: &[]awss3deployment.ISource{
			awss3deployment.Source_Asset(jsii.String("../frontend/out"), nil),
			awss3deployment.Source_JsonData(jsii.String("config.json"), config, nil),
		},
	})
}

// url is the https origin the browser loads and the value added to the API and bucket CORS lists.
func (f frontendHosting) url() string {
	return "https://" + *f.distribution.DistributionDomainName()
}
