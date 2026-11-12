package ecspresso_test

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
)

func TestUnmarshalJSONStrictKeys(t *testing.T) {
	src := `{
		"desiredCount": 2,
		"deploymentConfiguration": {"maximumPercent": 200},
		"loadBalancers": [],
		"monitoring": null,
		"unknownField": {"desiredcount": 1},
		"networkConfiguration": {
			"awsvpcConfiguration": {"subnets": ["subnet-1"], "unknown": true}
		}
	}`
	var sv ecspresso.Service
	if err := ecspresso.UnmarshalJSON([]byte(src), &sv, "test"); err != nil {
		t.Fatal(err)
	}
	want := ecspresso.Service{
		DesiredCount:            aws.Int32(2),
		DeploymentConfiguration: &types.DeploymentConfiguration{MaximumPercent: aws.Int32(200)},
		LoadBalancers:           []types.LoadBalancer{}, // an empty array must be kept
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{Subnets: []string{"subnet-1"}},
		},
	}
	if diff := cmp.Diff(want, sv, cmpopts.IgnoreUnexported(
		types.Service{}, types.DeploymentConfiguration{}, types.NetworkConfiguration{}, types.AwsVpcConfiguration{},
	)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestUnmarshalJSONMapKeys(t *testing.T) {
	// keys of maps are not field names. they must be kept as they are.
	src := `{
		"containerDefinitions": [{
			"name": "app",
			"dockerLabels": {"Foo": "1", "foo": "2"},
			"logConfiguration": {"logDriver": "awslogs", "options": {"awslogs-group": "g", "Mode": "non-blocking"}}
		}],
		"volumes": [{
			"name": "v",
			"dockerVolumeConfiguration": {"driverOpts": {"type": "nfs", "O": "x"}, "labels": {"Name": "v"}}
		}]
	}`
	var td ecspresso.TaskDefinitionInput
	if err := ecspresso.UnmarshalJSONForStruct([]byte(src), &td, "test"); err != nil {
		t.Fatal(err)
	}
	c := td.ContainerDefinitions[0]
	dv := td.Volumes[0].DockerVolumeConfiguration
	for name, d := range map[string]string{
		"dockerLabels": cmp.Diff(map[string]string{"Foo": "1", "foo": "2"}, c.DockerLabels),
		"options":      cmp.Diff(map[string]string{"awslogs-group": "g", "Mode": "non-blocking"}, c.LogConfiguration.Options),
		"driverOpts":   cmp.Diff(map[string]string{"type": "nfs", "O": "x"}, dv.DriverOpts),
		"labels":       cmp.Diff(map[string]string{"Name": "v"}, dv.Labels),
	} {
		if d != "" {
			t.Errorf("%s mismatch (-want +got):\n%s", name, d)
		}
	}
}

func TestUnmarshalJSONMiscasedKeys(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		v     any
		wants []string
	}{
		{
			name:  "lower case",
			src:   `{"desiredcount": 1}`,
			v:     &ecspresso.Service{},
			wants: []string{`unknown field "desiredcount" at /desiredcount: keys are case-sensitive, use "desiredCount"`},
		},
		{
			name:  "upper camel case",
			src:   `{"DesiredCount": 1}`,
			v:     &ecspresso.Service{},
			wants: []string{`unknown field "DesiredCount" at /DesiredCount: keys are case-sensitive, use "desiredCount"`},
		},
		{
			name: "all of the nested keys are reported",
			src:  `{"deploymentConfiguration": {"maximumpercent": 200}, "loadBalancers": [{"containerName": "app"}, {"ContainerName": "app"}]}`,
			v:    &ecspresso.Service{},
			wants: []string{
				`unknown field "maximumpercent" at /deploymentConfiguration/maximumpercent: keys are case-sensitive, use "maximumPercent"`,
				`unknown field "ContainerName" at /loadBalancers/1/ContainerName: keys are case-sensitive, use "containerName"`,
			},
		},
		{
			name:  "task definition",
			src:   `{"containerDefinitions": [{"Name": "app"}]}`,
			v:     &ecspresso.TaskDefinitionInput{},
			wants: []string{`unknown field "Name" at /containerDefinitions/0/Name: keys are case-sensitive, use "name"`},
		},
		{
			name:  "express definition",
			src:   `{"primaryContainer": {"Image": "nginx"}}`,
			v:     &ecspresso.ExpressGatewayService{},
			wants: []string{`unknown field "Image" at /primaryContainer/Image: keys are case-sensitive, use "image"`},
		},
		{
			name:  "overrides",
			src:   `{"containerOverrides": [{"name": "app", "Command": ["true"]}]}`,
			v:     &types.TaskOverride{},
			wants: []string{`unknown field "Command" at /containerOverrides/0/Command: keys are case-sensitive, use "command"`},
		},
		{
			name:  "config",
			src:   `{"region": "ap-northeast-1", "Cluster": "default", "codedeploy": {"Application_Name": "app"}}`,
			v:     &ecspresso.Config{},
			wants: []string{`use "cluster"`, `at /codedeploy/Application_Name: keys are case-sensitive, use "application_name"`},
		},
		{
			name:  "trailing value",
			src:   `{"cpu": "256"}{"memory": "512"}`,
			v:     &types.TaskOverride{},
			wants: []string{`unexpected value after the top-level value`},
		},
		{
			name:  "trailing garbage",
			src:   `{"cpu": "256"} x`,
			v:     &types.TaskOverride{},
			wants: []string{`invalid character`},
		},
		{
			name:  "duplicate keys",
			src:   `{"desiredCount": 1, "desiredCount": 2}`,
			v:     &ecspresso.Service{},
			wants: []string{`duplicate`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ecspresso.UnmarshalJSON([]byte(c.src), c.v, "test")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestUnmarshalJSONForStructMiscasedKeysToBeRemoved(t *testing.T) {
	// null values and empty arrays are removed, but their keys must be validated.
	for _, src := range []string{
		`{"family": "test", "ContainerDefinitions": []}`,
		`{"family": "test", "Cpu": null}`,
		`{"family": "test", "containerDefinitions": [{"name": "app", "Environment": []}]}`,
	} {
		t.Run(src, func(t *testing.T) {
			var td ecspresso.TaskDefinitionInput
			err := ecspresso.UnmarshalJSONForStruct([]byte(src), &td, "test")
			if err == nil || !strings.Contains(err.Error(), "keys are case-sensitive") {
				t.Errorf("expected an error of mis-cased key, got %v", err)
			}
		})
	}
	var td ecspresso.TaskDefinitionInput
	src := `{"family": "test", "containerDefinitions": [{"name": "app", "environment": [], "cpu": null}], "volumes": []}`
	if err := ecspresso.UnmarshalJSONForStruct([]byte(src), &td, "test"); err != nil {
		t.Fatal(err)
	}
	if td.Volumes != nil || td.ContainerDefinitions[0].Environment != nil {
		t.Errorf("empty arrays must be removed: %#v", td)
	}
}

func TestUnwrapTaskDefinition(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		wantErr string
	}{
		{name: "not wrapped", src: `{"family":"test"}`, want: `{"family":"test"}`},
		{name: "wrapped", src: `{"taskDefinition":{"family":"test"},"tags":[]}`, want: `{"family":"test"}`},
		{name: "mis-cased", src: `{"TaskDefinition":{"family":"test"}}`, wantErr: `use "taskDefinition"`},
		{name: "both", src: `{"taskDefinition":{"family":"a"},"TaskDefinition":{"family":"b"}}`, wantErr: `use "taskDefinition"`},
		{name: "duplicate", src: `{"taskDefinition":{"family":"a"},"taskDefinition":{"family":"b"}}`, wantErr: `duplicate`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ecspresso.UnwrapTaskDefinition([]byte(c.src))
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("expected error containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestServiceDefinitionDuplicateKeysWithHookDetails(t *testing.T) {
	// extracting hookDetails must not hide duplicate keys from the decoder.
	src := `{"desiredCount": 1, "desiredCount": 2, "deploymentConfiguration": {"lifecycleHooks": [{"hookDetails": {"a": 1}}]}}`
	b, _, err := ecspresso.ExtractHookDetails([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var sv ecspresso.Service
	if err := ecspresso.UnmarshalJSON(b, &sv, "test"); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected an error of duplicate key, got %v", err)
	}
}
