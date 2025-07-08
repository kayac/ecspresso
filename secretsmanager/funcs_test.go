package secretsmanager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/go-jsonnet"
	sm "github.com/kayac/ecspresso/v2/secretsmanager"
)

type mockSecretsManagerClient struct{}

var arnFmt = "arn:aws:secretsmanager:us-west-1:123456789012:secret:%s-deadbeef"

func (m *mockSecretsManagerClient) DescribeSecret(ctx context.Context, input *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	return &secretsmanager.DescribeSecretOutput{
		ARN: aws.String(fmt.Sprintf(arnFmt, *input.SecretId)),
	}, nil
}

func TestJsonnetNativeFuncs(t *testing.T) {
	app := sm.MockNewApp(&mockSecretsManagerClient{})
	funcs := app.JsonnetNativeFuncs(context.Background())
	vm := jsonnet.MakeVM()
	for _, f := range funcs {
		vm.NativeFunction(f)
	}
	out, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `
		local arn = std.native('secretsmanager_arn');
		arn('my-secret')
	`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	b, _ := json.Marshal(fmt.Sprintf(arnFmt, "my-secret"))
	expect := string(b)

	if strings.TrimSuffix(out, "\n") != expect {
		t.Fatalf("expected secretsmanager_arn function to return %s, got %s", expect, out)
	}
}

func TestJsonnetNativeFuncsSecretsManagerArnf(t *testing.T) {
	app := sm.MockNewApp(&mockSecretsManagerClient{})
	funcs := app.JsonnetNativeFuncs(context.Background())
	vm := jsonnet.MakeVM()
	for _, f := range funcs {
		vm.NativeFunction(f)
	}
	out, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `
		local arnf = std.native('secretsmanager_arnf');
		arnf('my-%s', 'secret')
	`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	b, _ := json.Marshal(fmt.Sprintf(arnFmt, "my-secret"))
	expect := string(b)

	if strings.TrimSuffix(out, "\n") != expect {
		t.Fatalf("expected secretsmanager_arnf function to return %s, got %s", expect, out)
	}
}

func TestFuncMapSecretsManagerArnf(t *testing.T) {
	app := sm.MockNewApp(&mockSecretsManagerClient{})
	funcMap := app.FuncMap(context.Background())
	
	tmpl, err := template.New("test").Funcs(funcMap).Parse(`{{ secretsmanager_arnf "my-%s" "secret" }}`)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}
	
	expected := fmt.Sprintf(arnFmt, "my-secret")
	if buf.String() != expected {
		t.Fatalf("expected secretsmanager_arnf function to return %s, got %s", expected, buf.String())
	}
}

func TestFuncMapSecretsManagerArnfMultipleArgs(t *testing.T) {
	app := sm.MockNewApp(&mockSecretsManagerClient{})
	funcMap := app.FuncMap(context.Background())
	
	tmpl, err := template.New("test").Funcs(funcMap).Parse(`{{ secretsmanager_arnf "%s-%s-%s" "my" "test" "secret" }}`)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}
	
	expected := fmt.Sprintf(arnFmt, "my-test-secret")
	if buf.String() != expected {
		t.Fatalf("expected secretsmanager_arnf function to return %s, got %s", expected, buf.String())
	}
}
