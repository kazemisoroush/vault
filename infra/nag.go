package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdklabs/cdk-nag-go/cdknag/v2"
)

// suppressNag records the cdk-nag rules Vault accepts by design or defers, with the reason for each.
func suppressNag(stack awscdk.Stack, healthRoute constructs.IConstruct) {
	cdknag.NagSuppressions_AddStackSuppressions(stack, &[]*cdknag.NagPackSuppression{
		{
			Id:     jsii.String("AwsSolutions-IAM4"),
			Reason: jsii.String("The Lambda uses the AWS managed basic execution role for CloudWatch Logs, the standard minimal logging policy."),
		},
		{
			Id:     jsii.String("AwsSolutions-IAM5"),
			Reason: jsii.String("Wildcards on the API role are scoped by design: S3 and DynamoDB object- and item-level access from the CDK grant helpers on the single Vault bucket and table, and bedrock:InvokeModel on Anthropic Claude foundation models and this account's inference profiles."),
		},
		{
			Id:     jsii.String("AwsSolutions-S1"),
			Reason: jsii.String("S3 server access logging is deferred to M4; single-user vault under the inside-the-box budget."),
		},
		{
			Id:     jsii.String("AwsSolutions-DDB3"),
			Reason: jsii.String("DynamoDB point-in-time recovery is deferred to M4; single-user vault."),
		},
		{
			Id:     jsii.String("AwsSolutions-COG2"),
			Reason: jsii.String("Cognito MFA is deferred; single-user vault."),
		},
		{
			Id:     jsii.String("AwsSolutions-COG8"),
			Reason: jsii.String("Cognito advanced security requires the paid Plus tier and is deferred; single-user vault."),
		},
		{
			Id:     jsii.String("AwsSolutions-APIG1"),
			Reason: jsii.String("HTTP API access logging is deferred to M4."),
		},
	}, jsii.Bool(true))

	cdknag.NagSuppressions_AddResourceSuppressions(healthRoute, &[]*cdknag.NagPackSuppression{
		{
			Id:     jsii.String("AwsSolutions-APIG4"),
			Reason: jsii.String("GET /health is an unauthenticated liveness probe that returns no data; every data route requires the Cognito JWT authorizer."),
		},
		{
			Id:     jsii.String("AwsSolutions-COG4"),
			Reason: jsii.String("GET /health is a liveness probe and needs no Cognito authorizer; data routes use it."),
		},
	}, jsii.Bool(true))
}
