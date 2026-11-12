package secretsmanager_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/go-jsonnet"
	sm "github.com/kayac/ecspresso/v3/secretsmanager"
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
	funcs := app.JsonnetNativeFuncs(t.Context())
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
