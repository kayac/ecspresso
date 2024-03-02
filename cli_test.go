package ecspresso_test

import (
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

var cliTests = []struct {
	args      []string
	sub       string
	option    *ecspresso.CLIOptions
	subOption any
	fn        func(*testing.T, any)
}{
	{
		args: []string{"status",
			"--config", "config.yml",
			"--debug",
			"--envfile", "tests/envfile",
			"--ext-str", "s1=v1",
			"--ext-str", "s2=v2",
			"--ext-code", "c1=123",
			"--ext-code", "c2=1+2",
			"--assume-role-arn", "arn:aws:iam::123456789012:role/exampleRole",
		},
		sub: "status",
		option: &ecspresso.CLIOptions{
			ConfigFilePath: "config.yml",
			Debug:          true,
			Envfile:        []string{"tests/envfile"},
			ExtStr:         map[string]string{"s1": "v1", "s2": "v2"},
			ExtCode:        map[string]string{"c1": "123", "c2": "1+2"},
			AssumeRoleARN:  "arn:aws:iam::123456789012:role/exampleRole",
		},
		subOption: &ecspresso.StatusOption{
			Events: 10,
		},
		fn: func(t *testing.T, _ any) {
			if v := os.Getenv("ECSPRESSO_TEST"); v != "ok" {
				t.Errorf("unexpected ECSPRESSO_TEST expected: %s, got: %s", "ok", v)
			}
		},
	},
	{
		args: []string{
			"--config", "config.yml",
			"--debug",
			"status",
			"--events=100",
		},
		sub: "status",
		option: &ecspresso.CLIOptions{
			ConfigFilePath: "config.yml",
			Debug:          true,
			ExtStr:         map[string]string{},
			ExtCode:        map[string]string{},
			AssumeRoleARN:  "",
		},
		subOption: &ecspresso.StatusOption{
			Events: 100,
		},
	},
	{
		args: []string{
			"--envfile", "tests/envfile",
			"status",
			"--envfile", "tests/envfile2",
		},
		sub: "status",
		fn: func(t *testing.T, _ any) {
			if v := os.Getenv("ECSPRESSO_TEST"); v != "ok2" {
				t.Errorf("unexpected ECSPRESSO_TEST expected: %s, got: %s", "ok2", v)
			}
			if v := os.Getenv("ECSPRESSO_TEST2"); v != "ok2" {
				t.Errorf("unexpected ECSPRESSO_TEST2 expected: %s, got: %s", "ok2", v)
			}
		},
	},
	{
		args: []string{"status"},
		sub:  "status",
		subOption: &ecspresso.StatusOption{
			Events: 10,
		},
	},
	{
		args: []string{"status", "--events=100"},
		sub:  "status",
		subOption: &ecspresso.StatusOption{
			Events: 100,
		},
	},
	{
		args: []string{"status", "--events", "20"},
		sub:  "status",
		subOption: &ecspresso.StatusOption{
			Events: 20,
		},
	},
	{
		args: []string{"deploy"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			DryRun:               false,
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   false,
			Revision:             0,
			ForceNewDeployment:   false,
			Wait:                 true,
			RollbackEvents:       "",
			UpdateService:        true,
			LatestTaskDefinition: false,
		},
	},
	{
		args: []string{"deploy", "--dry-run", "--tasks=10",
			"--skip-task-definition", "--revision=42", "--force-new-deployment",
			"--no-wait", "--latest-task-definition"},
		sub: "deploy",
		subOption: &ecspresso.DeployOption{
			DryRun:               true,
			DesiredCount:         ptr(int32(10)),
			SkipTaskDefinition:   true,
			Revision:             42,
			ForceNewDeployment:   true,
			Wait:                 false,
			RollbackEvents:       "",
			UpdateService:        true,
			LatestTaskDefinition: true,
		},
	},
	{
		args: []string{"deploy", "--resume-auto-scaling"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			SuspendAutoScaling:   nil,
			ResumeAutoScaling:    ptr(true),
			DryRun:               false,
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   false,
			Revision:             0,
			ForceNewDeployment:   false,
			Wait:                 true,
			RollbackEvents:       "",
			UpdateService:        true,
			LatestTaskDefinition: false,
		},
	},
	{
		args: []string{"deploy", "--suspend-auto-scaling"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			SuspendAutoScaling:   ptr(true),
			ResumeAutoScaling:    nil,
			DryRun:               false,
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   false,
			Revision:             0,
			ForceNewDeployment:   false,
			Wait:                 true,
			RollbackEvents:       "",
			UpdateService:        true,
			LatestTaskDefinition: false,
		},
	},
	{
		args: []string{"deploy", "--suspend-auto-scaling", "--auto-scaling-min=3", "--auto-scaling-max=10"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			SuspendAutoScaling:   ptr(true),
			AutoScalingMin:       ptr(int32(3)),
			AutoScalingMax:       ptr(int32(10)),
			ResumeAutoScaling:    nil,
			DryRun:               false,
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   false,
			ForceNewDeployment:   false,
			Wait:                 true,
			RollbackEvents:       "",
			UpdateService:        true,
			LatestTaskDefinition: false,
			Revision:             0,
		},
	},
	{
		args: []string{"scale", "--tasks=5"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:       false,
			DesiredCount: ptr(int32(5)),
			Wait:         true,
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				DesiredCount:         ptr(int32(5)),
				SkipTaskDefinition:   true,
				ForceNewDeployment:   false,
				Wait:                 true,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"scale", "--no-wait"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:       false,
			DesiredCount: ptr(int32(-1)),
			Wait:         false,
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				DesiredCount:         ptr(int32(-1)),
				SkipTaskDefinition:   true,
				ForceNewDeployment:   false,
				Wait:                 false,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"scale", "--suspend-auto-scaling"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:             false,
			DesiredCount:       ptr(int32(-1)),
			Wait:               true,
			SuspendAutoScaling: ptr(true),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				DesiredCount:         ptr(int32(-1)),
				SkipTaskDefinition:   true,
				ForceNewDeployment:   false,
				Wait:                 true,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
				SuspendAutoScaling:   ptr(true),
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"scale", "--resume-auto-scaling"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:            false,
			DesiredCount:      ptr(int32(-1)),
			Wait:              true,
			ResumeAutoScaling: ptr(true),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				DesiredCount:         ptr(int32(-1)),
				SkipTaskDefinition:   true,
				ForceNewDeployment:   false,
				Wait:                 true,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
				ResumeAutoScaling:    ptr(true),
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"scale", "--resume-auto-scaling", "--auto-scaling-min=3", "--auto-scaling-max=10"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:            false,
			DesiredCount:      ptr(int32(-1)),
			Wait:              true,
			ResumeAutoScaling: ptr(true),
			AutoScalingMin:    ptr(int32(3)),
			AutoScalingMax:    ptr(int32(10)),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				DesiredCount:         ptr(int32(-1)),
				SkipTaskDefinition:   true,
				ForceNewDeployment:   false,
				Wait:                 true,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
				ResumeAutoScaling:    ptr(true),
				AutoScalingMin:       ptr(int32(3)),
				AutoScalingMax:       ptr(int32(10)),
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"refresh"},
		sub:  "refresh",
		subOption: &ecspresso.RefreshOption{
			DryRun: false,
			Wait:   true,
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.RefreshOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				SkipTaskDefinition:   true,
				ForceNewDeployment:   true,
				Wait:                 true,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"refresh", "--no-wait"},
		sub:  "refresh",
		subOption: &ecspresso.RefreshOption{
			DryRun: false,
			Wait:   false,
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.RefreshOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               false,
				SkipTaskDefinition:   true,
				ForceNewDeployment:   true,
				Wait:                 false,
				RollbackEvents:       "",
				UpdateService:        false,
				LatestTaskDefinition: false,
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"rollback"},
		sub:  "rollback",
		subOption: &ecspresso.RollbackOption{
			DryRun:                   false,
			DeregisterTaskDefinition: true, // v2
			Wait:                     true,
			RollbackEvents:           "",
		},
	},
	{
		args: []string{"rollback", "--no-wait"},
		sub:  "rollback",
		subOption: &ecspresso.RollbackOption{
			DryRun:                   false,
			DeregisterTaskDefinition: true, // v2
			Wait:                     false,
			RollbackEvents:           "",
		},
	},
	{
		args: []string{"rollback", "--no-deregister-task-definition"},
		sub:  "rollback",
		subOption: &ecspresso.RollbackOption{
			DryRun:                   false,
			DeregisterTaskDefinition: false,
			Wait:                     true,
			RollbackEvents:           "",
		},
	},
	{
		args: []string{"delete"},
		sub:  "delete",
		subOption: &ecspresso.DeleteOption{
			DryRun:    false,
			Force:     false,
			Terminate: false,
		},
	},
	{
		args: []string{"delete", "--force"},
		sub:  "delete",
		subOption: &ecspresso.DeleteOption{
			DryRun:    false,
			Force:     true,
			Terminate: false,
		},
	},
	{
		args: []string{"delete", "--terminate"},
		sub:  "delete",
		subOption: &ecspresso.DeleteOption{
			DryRun:    false,
			Force:     false,
			Terminate: true,
		},
	},
	{
		args: []string{"run"},
		sub:  "run",
		subOption: &ecspresso.RunOption{
			DryRun:                 false,
			TaskDefinition:         "",
			Wait:                   true,
			Count:                  int32(1),
			WatchContainer:         "",
			PropagateTags:          "",
			TaskOverrideStr:        "",
			TaskOverrideFile:       "",
			SkipTaskDefinition:     false,
			LatestTaskDefinition:   false,
			Tags:                   "",
			WaitUntil:              "stopped",
			Revision:               ptr(int64(0)),
			ClientToken:            nil,
			EBSDeleteOnTermination: ptr(true),
		},
	},
	{
		args: []string{"run", "--no-wait", "--dry-run"},
		sub:  "run",
		subOption: &ecspresso.RunOption{
			DryRun:                 true,
			TaskDefinition:         "",
			Wait:                   false,
			Count:                  int32(1),
			WatchContainer:         "",
			PropagateTags:          "",
			TaskOverrideStr:        "",
			TaskOverrideFile:       "",
			SkipTaskDefinition:     false,
			LatestTaskDefinition:   false,
			Tags:                   "",
			WaitUntil:              "stopped",
			Revision:               ptr(int64(0)),
			ClientToken:            nil,
			EBSDeleteOnTermination: ptr(true),
		},
	},
	{
		args: []string{"run", "--task-def=foo.json", "--count", "2",
			"--watch-container", "app", "--propagate-tags", "SERVICE",
			"--overrides", `{"foo":"bar"}`,
			"--overrides-file", "overrides.json",
			"--latest-task-definition", "--tags", "KeyFoo=ValueFoo,KeyBar=ValueBar",
			"--wait-until", "running", "--revision", "1",
			"--client-token", "3abb3a41-c4dc-4c16-a3be-aaab729008a0",
		},
		sub: "run",
		subOption: &ecspresso.RunOption{
			DryRun:                 false,
			TaskDefinition:         "foo.json",
			Wait:                   true,
			Count:                  int32(2),
			WatchContainer:         "app",
			PropagateTags:          "SERVICE",
			TaskOverrideStr:        `{"foo":"bar"}`,
			TaskOverrideFile:       "overrides.json",
			SkipTaskDefinition:     false,
			LatestTaskDefinition:   true,
			Tags:                   "KeyFoo=ValueFoo,KeyBar=ValueBar",
			WaitUntil:              "running",
			Revision:               ptr(int64(1)),
			ClientToken:            ptr("3abb3a41-c4dc-4c16-a3be-aaab729008a0"),
			EBSDeleteOnTermination: ptr(true),
		},
	},
	{
		args: []string{"run", "--no-ebs-delete-on-termination"},
		sub:  "run",
		subOption: &ecspresso.RunOption{
			DryRun:                 false,
			TaskDefinition:         "",
			Wait:                   true,
			Count:                  int32(1),
			WatchContainer:         "",
			PropagateTags:          "",
			TaskOverrideStr:        "",
			TaskOverrideFile:       "",
			SkipTaskDefinition:     false,
			LatestTaskDefinition:   false,
			Tags:                   "",
			WaitUntil:              "stopped",
			Revision:               ptr(int64(0)),
			ClientToken:            nil,
			EBSDeleteOnTermination: ptr(false),
		},
	},
	{
		args: []string{"register"},
		sub:  "register",
		subOption: &ecspresso.RegisterOption{
			DryRun: false,
			Output: false,
		},
	},
	{
		args: []string{"register", "--output", "--dry-run"},
		sub:  "register",
		subOption: &ecspresso.RegisterOption{
			DryRun: true,
			Output: true,
		},
	},
	{
		args: []string{"deregister"},
		sub:  "deregister",
		subOption: &ecspresso.DeregisterOption{
			DryRun:   false,
			Revision: "",
			Keeps:    nil,
			Force:    false,
			Delete:   false,
		},
	},
	{
		args: []string{"deregister",
			"--dry-run", "--revision", "123", "--keeps", "23", "--force"},
		sub: "deregister",
		subOption: &ecspresso.DeregisterOption{
			DryRun:   true,
			Revision: "123",
			Keeps:    ptr(23),
			Force:    true,
			Delete:   false,
		},
	},
	{
		args: []string{"deregister",
			"--dry-run", "--revision", "latest", "--keeps", "23", "--force"},
		sub: "deregister",
		subOption: &ecspresso.DeregisterOption{
			DryRun:   true,
			Revision: "latest",
			Keeps:    ptr(23),
			Force:    true,
			Delete:   false,
		},
	},
	{
		args: []string{"deregister",
			"--dry-run", "--revision", "latest", "--keeps", "23", "--force", "--delete"},
		sub: "deregister",
		subOption: &ecspresso.DeregisterOption{
			DryRun:   true,
			Revision: "latest",
			Keeps:    ptr(23),
			Force:    true,
			Delete:   true,
		},
	},
	{
		args: []string{"revisions"},
		sub:  "revisions",
		subOption: &ecspresso.RevisionsOption{
			Revision: "",
			Output:   "table",
		},
	},
	{
		args: []string{"revisions", "--revision", "123", "--output", "json"},
		sub:  "revisions",
		subOption: &ecspresso.RevisionsOption{
			Revision: "123",
			Output:   "json",
		},
	},
	{
		args: []string{"revisions", "--revision", "current", "--output", "json"},
		sub:  "revisions",
		subOption: &ecspresso.RevisionsOption{
			Revision: "current",
			Output:   "json",
		},
	},
	{
		args: []string{"revisions", "--revision", "latest", "--output", "json"},
		sub:  "revisions",
		subOption: &ecspresso.RevisionsOption{
			Revision: "latest",
			Output:   "json",
		},
	},
	{
		args:      []string{"wait"},
		sub:       "wait",
		subOption: &ecspresso.WaitOption{},
	},
	{
		args: []string{"init", "--service", "myservice", "--config", "myconfig.yml"},
		sub:  "init",
		subOption: &ecspresso.InitOption{
			Region:                os.Getenv("AWS_REGION"),
			Cluster:               "default",
			Service:               "myservice",
			TaskDefinitionPath:    "ecs-task-def.json",
			ServiceDefinitionPath: "ecs-service-def.json",
			ForceOverwrite:        false,
			Jsonnet:               false,
		},
	},
	{
		args: []string{"init", "--service", "myservice", "--config", "myconfig.yml"},
		sub:  "init",
		option: &ecspresso.CLIOptions{
			ConfigFilePath: "myconfig.yml",
			Debug:          false,
			ExtStr:         map[string]string{},
			ExtCode:        map[string]string{},
		},
		subOption: &ecspresso.InitOption{
			Region:                os.Getenv("AWS_REGION"),
			Cluster:               "default",
			Service:               "myservice",
			TaskDefinitionPath:    "ecs-task-def.json",
			ServiceDefinitionPath: "ecs-service-def.json",
			ForceOverwrite:        false,
			Jsonnet:               false,
		},
	},
	{
		args: []string{"init", "--service", "myservice", "--config", "myconfig.jsonnet",
			"--cluster", "mycluster",
			"--task-definition-path", "taskdef.jsonnet",
			"--service-definition-path", "servicedef.jsonnet",
			"--force-overwrite", "--jsonnet",
		},
		sub: "init",
		subOption: &ecspresso.InitOption{
			Region:                os.Getenv("AWS_REGION"),
			Cluster:               "mycluster",
			Service:               "myservice",
			TaskDefinitionPath:    "taskdef.jsonnet",
			ServiceDefinitionPath: "servicedef.jsonnet",
			ForceOverwrite:        true,
			Jsonnet:               true,
		},
	},
	{
		args: []string{"init", "--task-definition=app:123", "--config", "myconfig.yml"},
		sub:  "init",
		subOption: &ecspresso.InitOption{
			Region:                os.Getenv("AWS_REGION"),
			Cluster:               "default",
			Service:               "",
			TaskDefinition:        "app:123",
			TaskDefinitionPath:    "ecs-task-def.json",
			ServiceDefinitionPath: "ecs-service-def.json",
			ForceOverwrite:        false,
			Jsonnet:               false,
		},
	},
	{
		args: []string{"diff"},
		sub:  "diff",
		subOption: &ecspresso.DiffOption{
			Unified: true,
		},
	},
	{
		args: []string{"diff", "--no-unified"},
		sub:  "diff",
		subOption: &ecspresso.DiffOption{
			Unified: false,
		},
	},
	{
		args: []string{"appspec"},
		sub:  "appspec",
		subOption: &ecspresso.AppSpecOption{
			TaskDefinition: "latest",
			UpdateService:  true,
		},
	},
	{
		args: []string{"appspec", "--task-definition", "current", "--no-update-service"},
		sub:  "appspec",
		subOption: &ecspresso.AppSpecOption{
			TaskDefinition: "current",
			UpdateService:  false,
		},
	},
	{
		args: []string{"verify"},
		sub:  "verify",
		subOption: &ecspresso.VerifyOption{
			GetSecrets: true,
			PutLogs:    true,
			Cache:      true,
		},
	},
	{
		args: []string{"verify", "--no-get-secrets", "--no-put-logs"},
		sub:  "verify",
		subOption: &ecspresso.VerifyOption{
			GetSecrets: false,
			PutLogs:    false,
			Cache:      true,
		},
	},
	{
		args: []string{"verify", "--no-get-secrets", "--no-put-logs", "--no-cache"},
		sub:  "verify",
		subOption: &ecspresso.VerifyOption{
			GetSecrets: false,
			PutLogs:    false,
			Cache:      false,
		},
	},
	{
		args: []string{"render", "config", "taskdef", "servicedef"},
		sub:  "render",
		subOption: &ecspresso.RenderOption{
			Targets: ptr([]string{"config", "taskdef", "servicedef"}),
			Jsonnet: false,
		},
	},
	{
		args: []string{"tasks"},
		sub:  "tasks",
		subOption: &ecspresso.TasksOption{
			ID:     "",
			Output: "table",
			Find:   false,
			Stop:   false,
			Force:  false,
			Trace:  false,
		},
	},
	{
		args: []string{"tasks", "--id", "abcdefff", "--output", "json",
			"--find", "--stop", "--force", "--trace",
		},
		sub: "tasks",
		subOption: &ecspresso.TasksOption{
			ID:     "abcdefff",
			Output: "json",
			Find:   true,
			Stop:   true,
			Force:  true,
			Trace:  true,
		},
	},
	{
		args: []string{"exec"},
		sub:  "exec",
		subOption: &ecspresso.ExecOption{
			ID:          "",
			Command:     "sh",
			Container:   "",
			LocalPort:   0,
			Port:        0,
			PortForward: false,
			Host:        "",
		},
	},
	{
		args: []string{"exec",
			"--id", "abcdefff",
			"--command", "ls -la",
			"--container", "mycontainer",
			"--local-port", "8080",
			"--port", "80",
			"--host", "example.com",
			"--port-forward",
		},
		sub: "exec",
		subOption: &ecspresso.ExecOption{
			ID:          "abcdefff",
			Command:     "ls -la",
			Container:   "mycontainer",
			LocalPort:   8080,
			Port:        80,
			PortForward: true,
			Host:        "example.com",
		},
	},
}

func TestParseCLIv2(t *testing.T) {
	for _, tt := range cliTests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			sub, opt, _, err := ecspresso.ParseCLIv2(tt.args)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if sub != tt.sub {
				t.Errorf("unexpected subcommand: expected %s, got %s", tt.sub, sub)
			}
			if tt.option != nil {
				if diff := cmp.Diff(tt.option, CLIOptionsGlobalOnly(opt)); diff != "" {
					t.Errorf("unexpected option: diff %s", diff)
				}
			}
			if tt.subOption != nil {
				if diff := cmp.Diff(opt.ForSubCommand(sub), tt.subOption); diff != "" {
					t.Errorf("unexpected subOption: diff %s", diff)
				}
			}
			if tt.fn != nil {
				tt.fn(t, opt.ForSubCommand(sub))
			}
		})
	}
}

func CLIOptionsGlobalOnly(opts *ecspresso.CLIOptions) *ecspresso.CLIOptions {
	return &ecspresso.CLIOptions{
		ConfigFilePath: opts.ConfigFilePath,
		Debug:          opts.Debug,
		ExtStr:         opts.ExtStr,
		ExtCode:        opts.ExtCode,
		Envfile:        opts.Envfile,
		AssumeRoleARN:  opts.AssumeRoleARN,
		Timeout:        opts.Timeout,
		FilterCommand:  opts.FilterCommand,
	}
}
