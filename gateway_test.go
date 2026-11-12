package ecspresso_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
)

var ignoreGatewayConnectorUnexported = cmpopts.IgnoreUnexported(
	types.GatewayConnector{},
	types.GatewayConnectorFilters{},
	types.GatewayConnectorHealthCheck{},
	types.GatewayConnectorHealthCheckMatcher{},
)

func testGatewayConnectors() []types.GatewayConnector {
	return []types.GatewayConnector{{PortName: aws.String("web")}}
}

func TestLoadServiceDefinitionGatewayConnectors(t *testing.T) {
	app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{ConfigFilePath: "tests/test.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	sv, err := app.LoadServiceDefinition("tests/sv-gateway.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []types.GatewayConnector{
		{
			PortName: aws.String("web"),
			Filters: &types.GatewayConnectorFilters{
				AccessTypes: []types.AccessType{types.AccessTypePrivate},
			},
			HealthCheck: &types.GatewayConnectorHealthCheck{
				HealthCheckPath:     aws.String("/health"),
				HealthCheckProtocol: types.GatewayConnectorHealthCheckProtocolHttp,
				Matcher: &types.GatewayConnectorHealthCheckMatcher{
					HttpCode: aws.String("200-299"),
				},
			},
		},
	}
	if diff := cmp.Diff(want, sv.GatewayConnectors, ignoreGatewayConnectorUnexported); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildServiceAttributesGatewayConnectors(t *testing.T) {
	app := &ecspresso.App{}
	tests := []struct {
		name string
		sv   *ecspresso.Service
		want []types.GatewayConnector
	}{
		{
			name: "connectors are passed",
			sv:   &ecspresso.Service{GatewayConnectors: testGatewayConnectors()},
			want: testGatewayConnectors(),
		},
		{
			name: "empty slice disconnects from gateways",
			sv:   &ecspresso.Service{},
			want: []types.GatewayConnector{},
		},
		{
			name: "not sent with CODE_DEPLOY deployment controller",
			sv: &ecspresso.Service{
				DeploymentController: &types.DeploymentController{Type: types.DeploymentControllerTypeCodeDeploy},
			},
			want: nil,
		},
		{
			name: "not sent in Express Mode",
			sv: &ecspresso.Service{
				ResourceManagementType: types.ResourceManagementTypeEcs,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := app.BuildServiceAttributes(tt.sv)
			if diff := cmp.Diff(tt.want, in.GatewayConnectors, ignoreGatewayConnectorUnexported); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
			if tt.want == nil && in.GatewayConnectors != nil {
				t.Errorf("expected nil, got %#v", in.GatewayConnectors)
			}
		})
	}
}

func TestDiffServicesGatewayConnectors(t *testing.T) {
	b := new(bytes.Buffer)
	opt := &ecspresso.DiffOption{}
	opt.SetWriter(b)
	color.NoColor = true

	newSv := func(gc []types.GatewayConnector) *ecspresso.Service {
		return &ecspresso.Service{
			LaunchType:        types.LaunchTypeFargate,
			GatewayConnectors: gc,
		}
	}

	t.Run("removal is shown", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffServices(t.Context(), newSv(nil), newSv(testGatewayConnectors()), "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if !diff {
			t.Fatal("expected diff but got none")
		}
		ds := b.String()
		for _, s := range []string{`-  "gatewayConnectors": [`, `-      "portName": "web"`} {
			if !strings.Contains(ds, s) {
				t.Errorf("expected diff to contain %q, got: %s", s, ds)
			}
		}
	})

	t.Run("same connectors", func(t *testing.T) {
		b.Reset()
		diff, err := ecspresso.DiffServices(t.Context(), newSv(testGatewayConnectors()), newSv(testGatewayConnectors()), "file", opt)
		if err != nil {
			t.Fatal(err)
		}
		if diff {
			t.Fatalf("unexpected diff: %s", b.String())
		}
	})
}
