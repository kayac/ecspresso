package ecspresso_test

import (
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
)

var cliTests = []struct {
	args      []string
	sub       string
	option    *ecspresso.Option
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
		option: &ecspresso.Option{
			ConfigFilePath: "config.yml",
			Debug:          true,
			ExtStr:         map[string]string{"s1": "v1", "s2": "v2"},
			ExtCode:        map[string]string{"c1": "123", "c2": "1+2"},
			InitOption:     nil,
			AssumeRoleARN:  "arn:aws:iam::123456789012:role/exampleRole",
		},
		subOption: &ecspresso.StatusOption{
			Events: ptr(10),
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
		option: &ecspresso.Option{
			ConfigFilePath: "config.yml",
			Debug:          true,
			ExtStr:         map[string]string{},
			ExtCode:        map[string]string{},
			InitOption:     nil,
			AssumeRoleARN:  "",
		},
		subOption: &ecspresso.StatusOption{
			Events: ptr(100),
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
			Events: ptr(10),
		},
	},
	{
		args: []string{"status", "--events=100"},
		sub:  "status",
		subOption: &ecspresso.StatusOption{
			Events: ptr(100),
		},
	},
	{
		args: []string{"status", "--events", "20"},
		sub:  "status",
		subOption: &ecspresso.StatusOption{
			Events: ptr(20),
		},
	},
	{
		args: []string{"deploy"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			DryRun:               ptr(false),
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   ptr(false),
			ForceNewDeployment:   ptr(false),
			NoWait:               ptr(false),
			RollbackEvents:       ptr(""),
			UpdateService:        ptr(true),
			LatestTaskDefinition: ptr(false),
		},
	},
	{
		args: []string{"deploy", "--dry-run", "--tasks=10",
			"--skip-task-definition", "--force-new-deployment",
			"--no-wait", "--latest-task-definition"},
		sub: "deploy",
		subOption: &ecspresso.DeployOption{
			DryRun:               ptr(true),
			DesiredCount:         ptr(int32(10)),
			SkipTaskDefinition:   ptr(true),
			ForceNewDeployment:   ptr(true),
			NoWait:               ptr(true),
			RollbackEvents:       ptr(""),
			UpdateService:        ptr(true),
			LatestTaskDefinition: ptr(true),
		},
	},
	{
		args: []string{"deploy", "--resume-auto-scaling"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			SuspendAutoScaling:   nil,
			ResumeAutoScaling:    ptr(true),
			DryRun:               ptr(false),
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   ptr(false),
			ForceNewDeployment:   ptr(false),
			NoWait:               ptr(false),
			RollbackEvents:       ptr(""),
			UpdateService:        ptr(true),
			LatestTaskDefinition: ptr(false),
		},
	},
	{
		args: []string{"deploy", "--suspend-auto-scaling"},
		sub:  "deploy",
		subOption: &ecspresso.DeployOption{
			SuspendAutoScaling:   ptr(true),
			ResumeAutoScaling:    nil,
			DryRun:               ptr(false),
			DesiredCount:         ptr(int32(-1)),
			SkipTaskDefinition:   ptr(false),
			ForceNewDeployment:   ptr(false),
			NoWait:               ptr(false),
			RollbackEvents:       ptr(""),
			UpdateService:        ptr(true),
			LatestTaskDefinition: ptr(false),
		},
	},
	{
		args: []string{"scale", "--tasks=5"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:       ptr(false),
			DesiredCount: ptr(int32(5)),
			NoWait:       ptr(false),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               ptr(false),
				DesiredCount:         ptr(int32(5)),
				SkipTaskDefinition:   ptr(true),
				ForceNewDeployment:   ptr(false),
				NoWait:               ptr(false),
				RollbackEvents:       ptr(""),
				UpdateService:        ptr(false),
				LatestTaskDefinition: ptr(false),
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"scale", "--suspend-auto-scaling"},
		sub:  "scale",
		subOption: &ecspresso.ScaleOption{
			DryRun:             ptr(false),
			DesiredCount:       ptr(int32(-1)),
			NoWait:             ptr(false),
			SuspendAutoScaling: ptr(true),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               ptr(false),
				DesiredCount:         ptr(int32(-1)),
				SkipTaskDefinition:   ptr(true),
				ForceNewDeployment:   ptr(false),
				NoWait:               ptr(false),
				RollbackEvents:       ptr(""),
				UpdateService:        ptr(false),
				LatestTaskDefinition: ptr(false),
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
			DryRun:            ptr(false),
			DesiredCount:      ptr(int32(-1)),
			NoWait:            ptr(false),
			ResumeAutoScaling: ptr(true),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.ScaleOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               ptr(false),
				DesiredCount:         ptr(int32(-1)),
				SkipTaskDefinition:   ptr(true),
				ForceNewDeployment:   ptr(false),
				NoWait:               ptr(false),
				RollbackEvents:       ptr(""),
				UpdateService:        ptr(false),
				LatestTaskDefinition: ptr(false),
				ResumeAutoScaling:    ptr(true),
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"refresh"},
		sub:  "refresh",
		subOption: &ecspresso.RefreshOption{
			DryRun: ptr(false),
			NoWait: ptr(false),
		},
		fn: func(t *testing.T, o any) {
			do := o.(*ecspresso.RefreshOption).DeployOption()
			if diff := cmp.Diff(do, ecspresso.DeployOption{
				DryRun:               ptr(false),
				SkipTaskDefinition:   ptr(true),
				ForceNewDeployment:   ptr(true),
				NoWait:               ptr(false),
				RollbackEvents:       ptr(""),
				UpdateService:        ptr(false),
				LatestTaskDefinition: ptr(false),
			}); diff != "" {
				t.Errorf("unexpected DeployOption (-want +got):\n%s", diff)
			}
		},
	},
	{
		args: []string{"rollback"},
		sub:  "rollback",
		subOption: &ecspresso.RollbackOption{
			DryRun:                   ptr(false),
			DeregisterTaskDefinition: ptr(true), // v2
			NoWait:                   ptr(false),
			RollbackEvents:           ptr(""),
		},
	},
	{
		args: []string{"rollback", "--no-deregister-task-definition"},
		sub:  "rollback",
		subOption: &ecspresso.RollbackOption{
			DryRun:                   ptr(false),
			DeregisterTaskDefinition: ptr(false),
			NoWait:                   ptr(false),
			RollbackEvents:           ptr(""),
		},
	},
	{
		args: []string{"delete"},
		sub:  "delete",
		subOption: &ecspresso.DeleteOption{
			DryRun:    ptr(false),
			Force:     ptr(false),
			Terminate: ptr(false),
		},
	},
	{
		args: []string{"delete", "--force"},
		sub:  "delete",
		subOption: &ecspresso.DeleteOption{
			DryRun:    ptr(false),
			Force:     ptr(true),
			Terminate: ptr(false),
		},
	},
	{
		args: []string{"delete", "--terminate"},
		sub:  "delete",
		subOption: &ecspresso.DeleteOption{
			DryRun:    ptr(false),
			Force:     ptr(false),
			Terminate: ptr(true),
		},
	},
	{
		args: []string{"run"},
		sub:  "run",
		subOption: &ecspresso.RunOption{
			DryRun:               ptr(false),
			TaskDefinition:       ptr(""),
			NoWait:               ptr(false),
			Count:                ptr(int32(1)),
			WatchContainer:       ptr(""),
			PropagateTags:        ptr(""),
			TaskOverrideStr:      ptr(""),
			TaskOverrideFile:     ptr(""),
			SkipTaskDefinition:   ptr(false),
			LatestTaskDefinition: ptr(false),
			Tags:                 ptr(""),
			WaitUntil:            ptr("stopped"),
			Revision:             ptr(int64(0)),
		},
	},
	{
		args: []string{"run", "--task-def=foo.json", "--count", "2",
			"--watch-container", "app", "--propagate-tags", "SERVICE",
			"--overrides", `{"foo":"bar"}`,
			"--overrides-file", "overrides.json",
			"--latest-task-definition", "--tags", "KeyFoo=ValueFoo,KeyBar=ValueBar",
			"--wait-until", "running", "--revision", "1",
		},
		sub: "run",
		subOption: &ecspresso.RunOption{
			DryRun:               ptr(false),
			TaskDefinition:       ptr("foo.json"),
			NoWait:               ptr(false),
			Count:                ptr(int32(2)),
			WatchContainer:       ptr("app"),
			PropagateTags:        ptr("SERVICE"),
			TaskOverrideStr:      ptr(`{"foo":"bar"}`),
			TaskOverrideFile:     ptr("overrides.json"),
			SkipTaskDefinition:   ptr(false),
			LatestTaskDefinition: ptr(true),
			Tags:                 ptr("KeyFoo=ValueFoo,KeyBar=ValueBar"),
			WaitUntil:            ptr("running"),
			Revision:             ptr(int64(1)),
		},
	},
	{
		args: []string{"register"},
		sub:  "register",
		subOption: &ecspresso.RegisterOption{
			DryRun: ptr(false),
			Output: ptr(false),
		},
	},
	{
		args: []string{"register", "--output", "--dry-run"},
		sub:  "register",
		subOption: &ecspresso.RegisterOption{
			DryRun: ptr(true),
			Output: ptr(true),
		},
	},
	{
		args: []string{"deregister"},
		sub:  "deregister",
		subOption: &ecspresso.DeregisterOption{
			DryRun:   ptr(false),
			Revision: ptr(int64(0)),
			Keeps:    ptr(0),
			Force:    ptr(false),
		},
	},
	{
		args: []string{"deregister",
			"--dry-run", "--revision", "123", "--keeps", "23", "--force"},
		sub: "deregister",
		subOption: &ecspresso.DeregisterOption{
			DryRun:   ptr(true),
			Revision: ptr(int64(123)),
			Keeps:    ptr(23),
			Force:    ptr(true),
		},
	},
	{
		args: []string{"revisions"},
		sub:  "revisions",
		subOption: &ecspresso.RevisionsOption{
			Revision: ptr(int64(0)),
			Output:   ptr("table"),
		},
	},
	{
		args: []string{"revisions", "--revision", "123", "--output", "json"},
		sub:  "revisions",
		subOption: &ecspresso.RevisionsOption{
			Revision: ptr(int64(123)),
			Output:   ptr("json"),
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
			Region:                ptr(os.Getenv("AWS_REGION")),
			Cluster:               ptr("default"),
			ConfigFilePath:        ptr("myconfig.yml"),
			Service:               ptr("myservice"),
			TaskDefinitionPath:    ptr("ecs-task-def.json"),
			ServiceDefinitionPath: ptr("ecs-service-def.json"),
			ForceOverwrite:        ptr(false),
			Jsonnet:               ptr(false),
		},
	},
	{
		args: []string{"init", "--service", "myservice", "--config", "myconfig.yml"},
		sub:  "init",
		option: &ecspresso.Option{
			InitOption: &ecspresso.InitOption{
				Region:                ptr(os.Getenv("AWS_REGION")),
				Cluster:               ptr("default"),
				ConfigFilePath:        ptr("myconfig.yml"),
				Service:               ptr("myservice"),
				TaskDefinitionPath:    ptr("ecs-task-def.json"),
				ServiceDefinitionPath: ptr("ecs-service-def.json"),
				ForceOverwrite:        ptr(false),
				Jsonnet:               ptr(false),
			},
			ConfigFilePath: "myconfig.yml",
			Debug:          false,
			ExtStr:         map[string]string{},
			ExtCode:        map[string]string{},
		},
		subOption: &ecspresso.InitOption{
			Region:                ptr(os.Getenv("AWS_REGION")),
			Cluster:               ptr("default"),
			ConfigFilePath:        ptr("myconfig.yml"),
			Service:               ptr("myservice"),
			TaskDefinitionPath:    ptr("ecs-task-def.json"),
			ServiceDefinitionPath: ptr("ecs-service-def.json"),
			ForceOverwrite:        ptr(false),
			Jsonnet:               ptr(false),
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
			Region:                ptr(os.Getenv("AWS_REGION")),
			Cluster:               ptr("mycluster"),
			ConfigFilePath:        ptr("myconfig.jsonnet"),
			Service:               ptr("myservice"),
			TaskDefinitionPath:    ptr("taskdef.jsonnet"),
			ServiceDefinitionPath: ptr("servicedef.jsonnet"),
			ForceOverwrite:        ptr(true),
			Jsonnet:               ptr(true),
		},
	},
	{
		args: []string{"diff"},
		sub:  "diff",
		subOption: &ecspresso.DiffOption{
			Unified: ptr(true),
		},
	},
	{
		args: []string{"diff", "--no-unified"},
		sub:  "diff",
		subOption: &ecspresso.DiffOption{
			Unified: ptr(false),
		},
	},
	{
		args: []string{"appspec"},
		sub:  "appspec",
		subOption: &ecspresso.AppSpecOption{
			TaskDefinition: ptr("latest"),
			UpdateService:  ptr(true),
		},
	},
	{
		args: []string{"appspec", "--task-definition", "current", "--no-update-service"},
		sub:  "appspec",
		subOption: &ecspresso.AppSpecOption{
			TaskDefinition: ptr("current"),
			UpdateService:  ptr(false),
		},
	},
	{
		args: []string{"verify"},
		sub:  "verify",
		subOption: &ecspresso.VerifyOption{
			GetSecrets: ptr(true),
			PutLogs:    ptr(true),
			Cache:      ptr(true),
		},
	},
	{
		args: []string{"verify", "--no-get-secrets", "--no-put-logs"},
		sub:  "verify",
		subOption: &ecspresso.VerifyOption{
			GetSecrets: ptr(false),
			PutLogs:    ptr(false),
			Cache:      ptr(true),
		},
	},
	{
		args: []string{"verify", "--no-get-secrets", "--no-put-logs", "--no-cache"},
		sub:  "verify",
		subOption: &ecspresso.VerifyOption{
			GetSecrets: ptr(false),
			PutLogs:    ptr(false),
			Cache:      ptr(false),
		},
	},
	{
		args: []string{"render", "config", "taskdef", "servicedef"},
		sub:  "render",
		subOption: &ecspresso.RenderOption{
			Targets: ptr([]string{"config", "taskdef", "servicedef"}),
			Jsonnet: ptr(false),
		},
	},
	{
		args: []string{"tasks"},
		sub:  "tasks",
		subOption: &ecspresso.TasksOption{
			ID:     ptr(""),
			Output: ptr("table"),
			Find:   ptr(false),
			Stop:   ptr(false),
			Force:  ptr(false),
			Trace:  ptr(false),
		},
	},
	{
		args: []string{"tasks", "--id", "abcdefff", "--output", "json",
			"--find", "--stop", "--force", "--trace",
		},
		sub: "tasks",
		subOption: &ecspresso.TasksOption{
			ID:     ptr("abcdefff"),
			Output: ptr("json"),
			Find:   ptr(true),
			Stop:   ptr(true),
			Force:  ptr(true),
			Trace:  ptr(true),
		},
	},
	{
		args: []string{"exec"},
		sub:  "exec",
		subOption: &ecspresso.ExecOption{
			ID:          ptr(""),
			Command:     ptr("sh"),
			Container:   ptr(""),
			LocalPort:   ptr(0),
			Port:        ptr(0),
			PortForward: ptr(false),
			Host:        ptr(""),
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
			ID:          ptr("abcdefff"),
			Command:     ptr("ls -la"),
			Container:   ptr("mycontainer"),
			LocalPort:   ptr(8080),
			Port:        ptr(80),
			PortForward: ptr(true),
			Host:        ptr("example.com"),
		},
	},
}

func TestParseCLIv2(t *testing.T) {
	cmpOption := cmp.Options{
		cmpopts.IgnoreUnexported(ecspresso.InitOption{}),
	}
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
				if diff := cmp.Diff(tt.option, opt.Option, cmpOption); diff != "" {
					t.Errorf("unexpected option: diff %s", diff)
				}
			}
			if tt.subOption != nil {
				if diff := cmp.Diff(opt.ForSubCommand(sub), tt.subOption, cmpOption); diff != "" {
					t.Errorf("unexpected subOption: diff %s", diff)
				}
			}
			if tt.fn != nil {
				tt.fn(t, opt.ForSubCommand(sub))
			}
		})
	}
}
