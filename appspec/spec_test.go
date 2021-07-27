package appspec_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/appspec"
	"github.com/kayac/go-config"
)

var expected = &appspec.AppSpec{
	Version: aws.String("0.0"),
	Resources: []*appspec.Resource{
		{
			TargetService: &appspec.TargetService{
				Type: aws.String("AWS::ECS::Service"),
				Properties: &appspec.Properties{
					TaskDefinition: aws.String("arn:aws:ecs:us-east-1:111222333444:task-definition/my-task-definition-family-name:1"),
					LoadBalancerInfo: &appspec.LoadBalancerInfo{
						ContainerName: aws.String("SampleApplicationName"),
						ContainerPort: aws.Int64(80),
					},
					PlatformVersion: aws.String("LATEST"),
					NetworkConfiguration: &appspec.NetworkConfiguration{
						AwsvpcConfiguration: &appspec.AwsVpcConfiguration{
							Subnets: []*string{
								aws.String("subnet-1234abcd"),
								aws.String("subnet-5678abcd"),
							},
							SecurityGroups: []*string{
								aws.String("sg-12345678"),
							},
							AssignPublicIp: aws.String("ENABLED"),
						},
					},
					CapacityProviderStrategy: []*appspec.CapacityProviderStrategy{
						{
							CapacityProvider: aws.String("FARGATE_SPOT"),
							Base: aws.Int64(1),
							Weight: aws.Int64(2),
						},
						{
							CapacityProvider: aws.String("FARGATE"),
							Base: aws.Int64(0),
							Weight: aws.Int64(1),
						},
					},
				},
			},
		},
	},
	Hooks: []*appspec.Hook{
		{BeforeInstall: "LambdaFunctionToValidateBeforeInstall"},
		{AfterInstall: "LambdaFunctionToValidateAfterTraffic"},
		{AfterAllowTestTraffic: "LambdaFunctionToValidateAfterTestTrafficStarts"},
		{BeforeAllowTraffic: "LambdaFunctionToValidateBeforeAllowingProductionTraffic"},
		{AfterAllowTraffic: "LambdaFunctionToValidateAfterAllowingProductionTraffic"},
	},
}

func TestAppSpec(t *testing.T) {
	var s appspec.AppSpec
	err := config.LoadWithEnv(&s, "test.yml")
	if err != nil {
		t.Error(err)
	}
	t.Log(s.String())
	if diff := cmp.Diff(&s, expected); diff != "" {
		t.Error(diff)
	}
}
