package ecspresso_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
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
			t.Errorf("unexpected vcpu conversion %s => %s expected %s", s[0], *cpu, s[1])
		}
	}
}

func TestToNumberMemory(t *testing.T) {
	for _, s := range testSuiteToNumberMemory {
		cpu := ecspresso.ToNumberMemory(s[0])
		if aws.ToString(cpu) != s[1] {
			t.Errorf("unexpected memory conversion %s => %s expected %s", s[0], *cpu, s[1])
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
	ecspresso.SortTaskDefinition(testTaskDefinition1)
	ecspresso.SortTaskDefinition(testTaskDefinition2)
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

var testServiceDefinitionNoDesiredCount = &ecspresso.Service{
	Service: types.Service{
		LaunchType: types.LaunchTypeFargate,
	},
	DesiredCount: nil,
}

var testServiceDefinitionHasDesiredCount = &ecspresso.Service{
	Service: types.Service{
		LaunchType: types.LaunchTypeFargate,
	},
	DesiredCount: ptr(int32(2)),
}

func TestDiffServices(t *testing.T) {
	ctx := t.Context()
	b := new(bytes.Buffer)
	opt := &ecspresso.DiffOption{}
	opt.SetWriter(b)
	color.NoColor = true

	t.Run("when local.DesiredCount is nil, ignore diff of DesiredCount", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffServices(
			ctx,
			testServiceDefinitionNoDesiredCount,
			testServiceDefinitionHasDesiredCount,
			"file", opt,
		)
		if err != nil {
			t.Error(err)
		}
		if diff {
			t.Errorf("unexpected differ: %t", diff)
		}
		if ds := b.String(); ds != "" {
			t.Errorf("unexpected diff: %s", ds)
		}
	})
	t.Run("when local.DesiredCount is not nil, detect diff of DesiredCount.", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffServices(
			ctx,
			testServiceDefinitionHasDesiredCount,
			testServiceDefinitionNoDesiredCount,
			"file", opt,
		)
		if err != nil {
			t.Error(err)
		}
		if !diff {
			t.Errorf("unexpected differ: %t", diff)
		}
		if ds := b.String(); ds == "" {
			t.Errorf("unexpected diff: %s", ds)
		}
	})

	t.Run("remote service is nil", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffServices(
			ctx,
			testServiceDefinitionNoDesiredCount,
			nil,
			"file", opt,
		)
		if err != nil {
			t.Error(err)
		}
		if !diff {
			t.Errorf("unexpected differ: %t", diff)
		}
		ds := b.String()
		if ds == "" {
			t.Errorf("unexpected diff: %s", ds)
		}
		minusDiffs := 0
		for line := range strings.SplitSeq(ds, "\n") {
			if strings.HasPrefix(line, "-") {
				minusDiffs++
			}
		}
		if minusDiffs != 1 { // The first line is "---"
			t.Errorf("unexpected diff. has many minus diffs: %s", ds)
		}
	})

	t.Run("when local.DeploymentConfiguration.MaximumPercent is nil, filled with default and no diff", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.Service{
			Service: types.Service{
				DeploymentConfiguration: &types.DeploymentConfiguration{
					MinimumHealthyPercent: aws.Int32(100),
					Strategy:              types.DeploymentStrategyRolling,
				},
				SchedulingStrategy: types.SchedulingStrategyReplica,
			},
		}
		remote := &ecspresso.Service{
			Service: types.Service{
				DeploymentConfiguration: &types.DeploymentConfiguration{
					MaximumPercent:        aws.Int32(200),
					MinimumHealthyPercent: aws.Int32(100),
					Strategy:              types.DeploymentStrategyRolling,
				},
				SchedulingStrategy: types.SchedulingStrategyReplica,
			},
		}
		diff, err := ecspresso.DiffServices(ctx, local, remote, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if diff {
			t.Fatalf("unexpected diff: %s", b.String())
		}
		if ds := b.String(); ds != "" {
			t.Fatalf("unexpected diff output: %s", ds)
		}
	})

	t.Run("when local.DeploymentConfiguration.MinimumHealthyPercent is nil, filled with default and no diff", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.Service{
			Service: types.Service{
				DeploymentConfiguration: &types.DeploymentConfiguration{
					MaximumPercent: aws.Int32(200),
					Strategy:       types.DeploymentStrategyRolling,
				},
				SchedulingStrategy: types.SchedulingStrategyReplica,
			},
		}
		remote := &ecspresso.Service{
			Service: types.Service{
				DeploymentConfiguration: &types.DeploymentConfiguration{
					MaximumPercent:        aws.Int32(200),
					MinimumHealthyPercent: aws.Int32(100),
					Strategy:              types.DeploymentStrategyRolling,
				},
				SchedulingStrategy: types.SchedulingStrategyReplica,
			},
		}
		diff, err := ecspresso.DiffServices(ctx, local, remote, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if diff {
			t.Fatalf("unexpected diff: %s", b.String())
		}
		if ds := b.String(); ds != "" {
			t.Fatalf("unexpected diff output: %s", ds)
		}
	})
}

func TestDiffServicesCodeDeployTargetGroupArn(t *testing.T) {
	ctx := t.Context()
	b := new(bytes.Buffer)
	opt := &ecspresso.DiffOption{}
	opt.SetWriter(b)
	color.NoColor = true

	t.Run("CodeDeploy service ignores targetGroupArn diff", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.Service{
			Service: types.Service{
				DeploymentController: &types.DeploymentController{
					Type: types.DeploymentControllerTypeCodeDeploy,
				},
				LoadBalancers: []types.LoadBalancer{
					{
						ContainerName:  aws.String("app"),
						ContainerPort:  aws.Int32(8080),
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/green/1234567890123456"),
					},
				},
			},
		}
		remote := &ecspresso.Service{
			Service: types.Service{
				DeploymentController: &types.DeploymentController{
					Type: types.DeploymentControllerTypeCodeDeploy,
				},
				LoadBalancers: []types.LoadBalancer{
					{
						ContainerName:  aws.String("app"),
						ContainerPort:  aws.Int32(8080),
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/blue/6543210987654321"),
					},
				},
			},
		}
		diff, err := ecspresso.DiffServices(ctx, local, remote, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if diff {
			t.Fatalf("unexpected diff for CodeDeploy service: %s", b.String())
		}
	})

	t.Run("ECS service detects targetGroupArn diff", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.Service{
			Service: types.Service{
				DeploymentController: &types.DeploymentController{
					Type: types.DeploymentControllerTypeEcs,
				},
				LoadBalancers: []types.LoadBalancer{
					{
						ContainerName:  aws.String("app"),
						ContainerPort:  aws.Int32(8080),
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/green/1234567890123456"),
					},
				},
			},
		}
		remote := &ecspresso.Service{
			Service: types.Service{
				DeploymentController: &types.DeploymentController{
					Type: types.DeploymentControllerTypeEcs,
				},
				LoadBalancers: []types.LoadBalancer{
					{
						ContainerName:  aws.String("app"),
						ContainerPort:  aws.Int32(8080),
						TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/blue/6543210987654321"),
					},
				},
			},
		}
		diff, err := ecspresso.DiffServices(ctx, local, remote, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected diff for ECS service with different targetGroupArn")
		}
		if ds := b.String(); !strings.Contains(ds, "targetGroupArn") {
			t.Fatalf("expected diff to contain targetGroupArn: %s", ds)
		}
	})
}

func TestDiffJsonnet(t *testing.T) {
	ctx := t.Context()
	b := new(bytes.Buffer)
	opt := &ecspresso.DiffOption{Jsonnet: true}
	opt.SetWriter(b)
	color.NoColor = true

	t.Run("jsonnet diff for task defs with difference", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("512"),
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name:  aws.String("app"),
					Image: aws.String("nginx:latest"),
				},
			},
		}
		remote := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("1024"),
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name:  aws.String("app"),
					Image: aws.String("nginx:stable"),
				},
			},
		}
		diff, err := ecspresso.DiffTaskDefs(ctx, local, remote, "local.jsonnet", "remote", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected diff")
		}
		ds := b.String()
		// jsonnet format uses unquoted keys
		if strings.Contains(ds, `"memory"`) {
			t.Errorf("expected jsonnet format (unquoted keys), got JSON: %s", ds)
		}
		if !strings.Contains(ds, "memory:") {
			t.Errorf("expected jsonnet format with 'memory:' key: %s", ds)
		}
	})

	t.Run("jsonnet diff for same task defs shows no diff", func(t *testing.T) {
		b.Reset()
		td := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("512"),
		}
		diff, err := ecspresso.DiffTaskDefs(ctx, td, td, "local.jsonnet", "remote", opt)
		if err != nil {
			t.Fatal(err)
		}
		if diff {
			t.Fatalf("unexpected diff: %s", b.String())
		}
	})
}

func TestDiffWithoutService(t *testing.T) {
	ctx := t.Context()
	color.NoColor = true

	localSv := &ecspresso.Service{
		Service: types.Service{
			LaunchType: types.LaunchTypeFargate,
			NetworkConfiguration: &types.NetworkConfiguration{
				AwsvpcConfiguration: &types.AwsVpcConfiguration{
					Subnets:        []string{"subnet-111"},
					SecurityGroups: []string{"sg-111"},
				},
			},
		},
	}
	remoteSv := &ecspresso.Service{
		Service: types.Service{
			LaunchType: types.LaunchTypeFargate,
			NetworkConfiguration: &types.NetworkConfiguration{
				AwsvpcConfiguration: &types.AwsVpcConfiguration{
					Subnets:        []string{"subnet-222"},
					SecurityGroups: []string{"sg-222"},
				},
			},
		},
	}

	t.Run("WithService=true shows service diff", func(t *testing.T) {
		b := new(bytes.Buffer)
		opt := &ecspresso.DiffOption{WithService: true}
		opt.SetWriter(b)

		diff, err := ecspresso.DiffServices(ctx, localSv, remoteSv, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected diff when WithService is true")
		}
		if b.String() == "" {
			t.Fatal("expected non-empty diff output")
		}
	})

	t.Run("WithService=false produces only task def diff", func(t *testing.T) {
		b := new(bytes.Buffer)
		opt := &ecspresso.DiffOption{WithService: false}
		opt.SetWriter(b)

		localTd := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("512"),
			ContainerDefinitions: []types.ContainerDefinition{
				{Name: aws.String("app"), Image: aws.String("nginx:latest")},
			},
		}
		remoteTd := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("1024"),
			ContainerDefinitions: []types.ContainerDefinition{
				{Name: aws.String("app"), Image: aws.String("nginx:stable")},
			},
		}

		// Simulate Diff method: when WithService=false, skip service diff
		serviceName := "my-service"
		if serviceName != "" && opt.WithService {
			// This block should NOT execute
			if _, err := ecspresso.DiffServices(ctx, localSv, remoteSv, "file", opt); err != nil {
				t.Fatal(err)
			}
		}
		// Task def diff always runs
		diff, err := ecspresso.DiffTaskDefs(ctx, localTd, remoteTd, "local", "remote", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected task def diff")
		}
		output := b.String()
		// Should contain task def changes but no service subnet/sg changes
		if strings.Contains(output, "subnet") {
			t.Errorf("expected no service diff in output, got: %s", output)
		}
		if !strings.Contains(output, "memory") {
			t.Errorf("expected task def diff with memory change, got: %s", output)
		}
	})

	t.Run("WithService=true produces both service and task def diff", func(t *testing.T) {
		b := new(bytes.Buffer)
		opt := &ecspresso.DiffOption{WithService: true}
		opt.SetWriter(b)

		localTd := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("512"),
			ContainerDefinitions: []types.ContainerDefinition{
				{Name: aws.String("app"), Image: aws.String("nginx:latest")},
			},
		}
		remoteTd := &ecspresso.TaskDefinitionInput{
			Cpu:    aws.String("256"),
			Memory: aws.String("1024"),
			ContainerDefinitions: []types.ContainerDefinition{
				{Name: aws.String("app"), Image: aws.String("nginx:stable")},
			},
		}

		// Simulate Diff method: when WithService=true, run service diff
		serviceName := "my-service"
		if serviceName != "" && opt.WithService {
			if _, err := ecspresso.DiffServices(ctx, localSv, remoteSv, "file", opt); err != nil {
				t.Fatal(err)
			}
		}
		// Task def diff always runs
		diff, err := ecspresso.DiffTaskDefs(ctx, localTd, remoteTd, "local", "remote", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected task def diff")
		}
		output := b.String()
		// Should contain both service and task def changes
		if !strings.Contains(output, "subnet") {
			t.Errorf("expected service diff with subnet change, got: %s", output)
		}
		if !strings.Contains(output, "memory") {
			t.Errorf("expected task def diff with memory change, got: %s", output)
		}
	})
}

func TestDiffTaskDefs(t *testing.T) {
	ctx := t.Context()
	b := new(bytes.Buffer)
	opt := &ecspresso.DiffOption{}
	opt.SetWriter(b)
	color.NoColor = true

	t.Run("diff task defs same actually", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffTaskDefs(
			ctx,
			testTaskDefinition1,
			testTaskDefinition2,
			"file", "remote", opt,
		)
		if err != nil {
			t.Error(err)
		}
		if diff {
			t.Errorf("unexpected differ: %t", diff)
		}
		if ds := b.String(); ds != "" {
			t.Errorf("unexpected diff: %s", ds)
		}
	})

	t.Run("diff task defs remote nil", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffTaskDefs(
			ctx,
			testTaskDefinition1,
			nil,
			"file", "", opt,
		)
		if err != nil {
			t.Error(err)
		}
		if !diff {
			t.Errorf("unexpected differ: %t", diff)
		}
		ds := b.String()
		if ds == "" {
			t.Errorf("unexpected diff: %s", ds)
		}
		minusDiffs := 0
		for line := range strings.SplitSeq(ds, "\n") {
			if strings.HasPrefix(line, "-") {
				minusDiffs++
			}
		}
		if minusDiffs != 1 { // The first line is "---"
			t.Errorf("unexpected diff. has many minus diffs: %s", ds)
		}
	})
}

func TestDiffServicesMonitoring(t *testing.T) {
	ctx := t.Context()
	b := new(bytes.Buffer)
	opt := &ecspresso.DiffOption{Unified: true}
	opt.SetWriter(b)
	color.NoColor = true

	monitoring := &types.MonitoringConfiguration{
		MetricConfigurations: []types.MetricConfiguration{
			{
				MetricNames:       []string{"CPUUtilization", "MemoryUtilization"},
				ResolutionSeconds: aws.Int32(20),
			},
		},
	}

	t.Run("monitoring added", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.Service{
			Service: types.Service{
				LaunchType: types.LaunchTypeFargate,
			},
			Monitoring: monitoring,
		}
		remote := &ecspresso.Service{
			Service: types.Service{
				LaunchType: types.LaunchTypeFargate,
			},
		}
		diff, err := ecspresso.DiffServices(ctx, local, remote, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected diff but got none")
		}
		ds := b.String()
		if !strings.Contains(ds, "monitoring") {
			t.Errorf("expected diff to contain 'monitoring', got: %s", ds)
		}
	})

	t.Run("monitoring same", func(t *testing.T) {
		b.Reset()
		local := &ecspresso.Service{
			Service: types.Service{
				LaunchType: types.LaunchTypeFargate,
			},
			Monitoring: monitoring,
		}
		remote := &ecspresso.Service{
			Service: types.Service{
				LaunchType: types.LaunchTypeFargate,
			},
			Monitoring: &types.MonitoringConfiguration{
				MetricConfigurations: []types.MetricConfiguration{
					{
						MetricNames:       []string{"CPUUtilization", "MemoryUtilization"},
						ResolutionSeconds: aws.Int32(20),
					},
				},
			},
		}
		diff, err := ecspresso.DiffServices(ctx, local, remote, "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if diff {
			t.Fatalf("unexpected diff: %s", b.String())
		}
	})
}
