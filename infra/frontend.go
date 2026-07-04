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

	// The static export writes one index.html per route (index.html, login/index.html).
	// CloudFront only appends the default root object for "/", not for sub-paths, so this
	// viewer-request function rewrites directory and extensionless URIs to their index.html.
	// Without it, "/login/" fell back to the root index.html and served the wrong page.
	rewrite := awscloudfront.NewFunction(stack, jsii.String("WebRewrite"), &awscloudfront.FunctionProps{
		Runtime: awscloudfront.FunctionRuntime_JS_2_0(),
		Code: awscloudfront.FunctionCode_FromInline(jsii.String(
			"function handler(event){var r=event.request;var u=r.uri;" +
				"if(u.endsWith('/')){r.uri=u+'index.html';}" +
				"else if(u.lastIndexOf('.')<u.lastIndexOf('/')){r.uri=u+'/index.html';}" +
				"return r;}",
		)),
	})

	distribution := awscloudfront.NewDistribution(stack, jsii.String("WebCdn"), &awscloudfront.DistributionProps{
		DefaultRootObject: jsii.String("index.html"),
		DefaultBehavior: &awscloudfront.BehaviorOptions{
			Origin:               awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(bucket, nil),
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			FunctionAssociations: &[]*awscloudfront.FunctionAssociation{
				{Function: rewrite, EventType: awscloudfront.FunctionEventType_VIEWER_REQUEST},
			},
		},
		PriceClass: awscloudfront.PriceClass_PRICE_CLASS_100,
		// A missing object returns the static 404 page with a real 404, so a missing asset is
		// not masked as a 200 of index.html.
		ErrorResponses: &[]*awscloudfront.ErrorResponse{
			{HttpStatus: jsii.Number(403), ResponseHttpStatus: jsii.Number(404), ResponsePagePath: jsii.String("/404.html")},
			{HttpStatus: jsii.Number(404), ResponseHttpStatus: jsii.Number(404), ResponsePagePath: jsii.String("/404.html")},
		},
	})

	return frontendHosting{bucket: bucket, distribution: distribution}
}

// webConfig is the runtime config written to config.json. Its keys must match the AppConfig
// type in frontend/lib/config.ts.
type webConfig struct {
	APIURL            *string `json:"apiUrl"`
	CognitoUserPoolID *string `json:"cognitoUserPoolId"`
	CognitoClientID   *string `json:"cognitoClientId"`
}

// deploy uploads the built static site plus a config.json rendered from the stack outputs,
// then invalidates the distribution so the new version is served immediately.
func (f frontendHosting) deploy(stack awscdk.Stack, apiURL, userPoolID, clientID *string) {
	config := webConfig{APIURL: apiURL, CognitoUserPoolID: userPoolID, CognitoClientID: clientID}
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

// URL is the https origin the browser loads and the value added to the API and bucket CORS lists.
func (f frontendHosting) URL() string {
	return "https://" + *f.distribution.DistributionDomainName()
}
