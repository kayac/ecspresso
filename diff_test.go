package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/kayac/ecspresso"
)

var testSuiteToNumberCPU = [][]string{
	{"256", "256"},
	{"0.25 vCPU", "256"},
	{"0.5vcpu", "512"},
	{"4 vcpu", "4096"},
}

func TestToNumberCPU(t *testing.T) {
	for _, s := range testSuiteToNumberCPU {
		cpu := ecspresso.ToNumberCPU(s[0])
		if !ecspresso.EqualString(cpu, s[1]) {
			t.Errorf("unexpected vcpu convertion %s => %s expected %s", s[0], *cpu, s[1])
		}
	}
}

var testTaskDefinition1 = &ecs.TaskDefinition{
	Cpu: aws.String("0.25 vCPU"),
	ContainerDefinitions: []*ecs.ContainerDefinition{
		{
			Name:  aws.String("app"),
			Image: aws.String("debian:buster"),
			Environment: []*ecs.KeyValuePair{
				{
					Name:  aws.String("TZ"),
					Value: aws.String("UTC"),
				},
				{
					Name:  aws.String("LANG"),
					Value: aws.String("en_US"),
				},
			},
		},
		{
			Name:  aws.String("web"),
			Image: aws.String("nginx:latest"),
		},
	},
}

var testTaskDefinition2 = &ecs.TaskDefinition{
	Cpu: aws.String("256"),
	ContainerDefinitions: []*ecs.ContainerDefinition{
		{
			Name:  aws.String("web"),
			Image: aws.String("nginx:latest"),
		},
		{
			Cpu:   aws.Int64(0),
			Name:  aws.String("app"),
			Image: aws.String("debian:buster"),
			Environment: []*ecs.KeyValuePair{
				{
					Name:  aws.String("LANG"),
					Value: aws.String("en_US"),
				},
				{
					Name:  aws.String("TZ"),
					Value: aws.String("UTC"),
				},
			},
		},
	},
	Volumes: []*ecs.Volume{},
}

func TestTaskDefinitionDiffer(t *testing.T) {
	ecspresso.SortTaskDefinitionForDiff(testTaskDefinition1)
	ecspresso.SortTaskDefinitionForDiff(testTaskDefinition2)
	if ecspresso.MarshalJSONString(testTaskDefinition1) != ecspresso.MarshalJSONString(testTaskDefinition2) {
		t.Error("failed to sortTaskDefinitionForDiff")
		t.Log(testTaskDefinition1.String())
		t.Log(testTaskDefinition2.String())
	}
}

var testServiceDefinition1 = &ecs.Service{
	LaunchType: aws.String("FARGATE"),
	NetworkConfiguration: &ecs.NetworkConfiguration{
		AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
			Subnets: []*string{
				aws.String("subnet-876543210"),
				aws.String("subnet-012345678"),
			},
			SecurityGroups: []*string{
				aws.String("sg-99999999"),
				aws.String("sg-11111111"),
			},
		},
	},
}

var testServiceDefinition2 = &ecs.Service{
	DeploymentConfiguration: &ecs.DeploymentConfiguration{
		MaximumPercent:        aws.Int64(200),
		MinimumHealthyPercent: aws.Int64(100),
	},
	NetworkConfiguration: &ecs.NetworkConfiguration{
		AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
			Subnets: []*string{
				aws.String("subnet-012345678"),
				aws.String("subnet-876543210"),
			},
			SecurityGroups: []*string{
				aws.String("sg-11111111"),
				aws.String("sg-99999999"),
			},
			AssignPublicIp: aws.String("DISABLED"),
		},
	},
	LaunchType:      aws.String("FARGATE"),
	PlatformVersion: aws.String("LATEST"),
}

func TestServiceDefinitionDiffer(t *testing.T) {
	ecspresso.SortServiceDefinitionForDiff(testServiceDefinition1)
	ecspresso.SortServiceDefinitionForDiff(testServiceDefinition2)
	if ecspresso.MarshalJSONString(testServiceDefinition1) != ecspresso.MarshalJSONString(testServiceDefinition2) {
		t.Error("failed to SortTaskDefinitionForDiff")
		t.Log(testServiceDefinition1.String())
		t.Log(testServiceDefinition2.String())
	}
}
