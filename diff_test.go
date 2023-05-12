package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
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
		if aws.ToString(cpu) != s[1] {
			t.Errorf("unexpected vcpu convertion %s => %s expected %s", s[0], *cpu, s[1])
		}
	}
}

func TestToNumberMemory(t *testing.T) {
	for _, s := range testSuiteToNumberMemory {
		cpu := ecspresso.ToNumberMemory(s[0])
		if aws.ToString(cpu) != s[1] {
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
	td1, _ := ecspresso.MarshalJSONForAPI(testTaskDefinition1)
	td2, _ := ecspresso.MarshalJSONForAPI(testTaskDefinition2)
	if diff := cmp.Diff(td1, td2); diff != "" {
		t.Error("failed to sortTaskDefinitionForDiff", diff)
		t.Log(string(td1))
		t.Log(string(td2))
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
		Tags: []types.Tag{
			{
				Key:   aws.String("Environment"),
				Value: aws.String("Dev"),
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
		LaunchType:         types.LaunchTypeFargate,
		PlatformVersion:    aws.String("LATEST"),
		SchedulingStrategy: types.SchedulingStrategyReplica,
		Tags: []types.Tag{
			{
				Key:   aws.String("Environment"),
				Value: aws.String("Dev"),
			},
		},
	},
}

func TestServiceDefinitionDiffer(t *testing.T) {
	sv1 := ecspresso.ServiceDefinitionForDiff(testServiceDefinition1)
	sv2 := ecspresso.ServiceDefinitionForDiff(testServiceDefinition2)
	sv1Bytes, _ := ecspresso.MarshalJSONForAPI(sv1)
	sv2Bytes, _ := ecspresso.MarshalJSONForAPI(sv2)
	if diff := cmp.Diff(sv1Bytes, sv2Bytes); diff != "" {
		t.Error("failed to SortTaskDefinitionForDiff", diff)
	}
}
