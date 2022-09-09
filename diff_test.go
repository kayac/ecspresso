package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/kayac/ecspresso"
)

var testSuiteToNumberCPU = [][]string{
	{"256", "256"},
	{"0.25 vCPU", "256"},
	{"0.5vcpu", "512"},
	{"4 vcpu", "4096"},
}

var testSuiteToNumberMemory = [][]string{
	{"512", "512"},
	{"0.5 GB", "512"},
	{"4GB", "4096"},
}

func TestToNumberCPU(t *testing.T) {
	for _, s := range testSuiteToNumberCPU {
		cpu := ecspresso.ToNumberCPU(s[0])
		if !ecspresso.EqualString(cpu, s[1]) {
			t.Errorf("unexpected vcpu convertion %s => %s expected %s", s[0], *cpu, s[1])
		}
	}
}

func TestToNumberMemory(t *testing.T) {
	for _, s := range testSuiteToNumberMemory {
		cpu := ecspresso.ToNumberMemory(s[0])
		if !ecspresso.EqualString(cpu, s[1]) {
			t.Errorf("unexpected memory convertion %s => %s expected %s", s[0], *cpu, s[1])
		}
	}
}

var testTaskDefinition1 = &ecspresso.TaskDefinitionInput{
	Cpu:    aws.String("0.25 vCPU"),
	Memory: aws.String("1 GB"),
	ContainerDefinitions: []types.ContainerDefinition{
		{
			Name:  aws.String("app"),
			Image: aws.String("debian:buster"),
			Environment: []types.KeyValuePair{
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
			Cpu:   0,
			Name:  aws.String("web"),
			Image: aws.String("nginx:latest"),
		},
	},
	ProxyConfiguration: &types.ProxyConfiguration{
		ContainerName: aws.String("envoy"),
		Properties: []types.KeyValuePair{
			{
				Name:  aws.String("ProxyIngressPort"),
				Value: aws.String("15000"),
			},
			{
				Name:  aws.String("ProxyEgressPort"),
				Value: aws.String("15001"),
			},
		},
	},
	Tags: []types.Tag{
		{
			Key:   aws.String("AppVersion"),
			Value: aws.String("v1"),
		}, {
			Key:   aws.String("Environment"),
			Value: aws.String("Dev"),
		},
	},
}

var testTaskDefinition2 = &ecspresso.TaskDefinitionInput{
	Cpu:    aws.String("256"),
	Memory: aws.String("1024"),
	ContainerDefinitions: []types.ContainerDefinition{
		{
			Name:  aws.String("web"),
			Image: aws.String("nginx:latest"),
		},
		{
			Name:  aws.String("app"),
			Image: aws.String("debian:buster"),
			Environment: []types.KeyValuePair{
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
	Volumes: []types.Volume{},
	ProxyConfiguration: &types.ProxyConfiguration{
		ContainerName: aws.String("envoy"),
		Properties: []types.KeyValuePair{
			{
				Name:  aws.String("ProxyEgressPort"),
				Value: aws.String("15001"),
			},
			{
				Name:  aws.String("ProxyIngressPort"),
				Value: aws.String("15000"),
			},
		},
	},
	Tags: []types.Tag{
		{
			Key:   aws.String("Environment"),
			Value: aws.String("Dev"),
		}, {
			Key:   aws.String("AppVersion"),
			Value: aws.String("v1"),
		},
	},
}

func TestTaskDefinitionDiffer(t *testing.T) {
	ecspresso.SortTaskDefinitionForDiff(testTaskDefinition1)
	ecspresso.SortTaskDefinitionForDiff(testTaskDefinition2)
	if ecspresso.MarshalJSONString(testTaskDefinition1) != ecspresso.MarshalJSONString(testTaskDefinition2) {
		t.Error("failed to sortTaskDefinitionForDiff")
		t.Log(testTaskDefinition1)
		t.Log(testTaskDefinition2)
	}
}

var testServiceDefinition1 = &ecspresso.Service{
	Service: types.Service{
		LaunchType: types.LaunchTypeFargate,
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets: []string{
					"subnet-876543210",
					"subnet-012345678",
				},
				SecurityGroups: []string{
					"sg-99999999",
					"sg-11111111",
				},
			},
		},
	},
}

var testServiceDefinition2 = &ecspresso.Service{
	Service: types.Service{
		DeploymentConfiguration: &types.DeploymentConfiguration{
			MaximumPercent:        aws.Int32(200),
			MinimumHealthyPercent: aws.Int32(100),
		},
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets: []string{
					"subnet-012345678",
					"subnet-876543210",
				},
				SecurityGroups: []string{
					"sg-11111111",
					"sg-99999999",
				},
				AssignPublicIp: types.AssignPublicIpDisabled,
			},
		},
		LaunchType:      types.LaunchTypeFargate,
		PlatformVersion: aws.String("LATEST"),
	},
}

func TestServiceDefinitionDiffer(t *testing.T) {
	ecspresso.SortServiceDefinitionForDiff(testServiceDefinition1)
	ecspresso.SortServiceDefinitionForDiff(testServiceDefinition2)
	if ecspresso.MarshalJSONString(testServiceDefinition1) != ecspresso.MarshalJSONString(testServiceDefinition2) {
		t.Error("failed to SortTaskDefinitionForDiff")
		t.Log(testServiceDefinition1)
		t.Log(testServiceDefinition2)
	}
}
