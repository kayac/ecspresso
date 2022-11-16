package ssm_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2/ssm"
)

type mockSSM struct {
	getParameter func(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error)
}

func (m mockSSM) GetParameter(ctx context.Context, input *awsssm.GetParameterInput, opts ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
	return m.getParameter(input)
}

func newMockApp(getParameter func(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error)) *ssm.App {
	return ssm.MockNew(mockSSM{getParameter: getParameter})
}

func mockGetParameter(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error) {
	switch *input.Name {
	case "/string":
		return &awsssm.GetParameterOutput{
			Parameter: &types.Parameter{
				Name:  input.Name,
				Type:  types.ParameterTypeString,
				Value: aws.String("string value"),
			},
		}, nil
	case "/stringlist":
		return &awsssm.GetParameterOutput{
			Parameter: &types.Parameter{
				Name:  input.Name,
				Type:  types.ParameterTypeStringList,
				Value: aws.String("stringlist value 1,stringlist value 2"),
			},
		}, nil
	case "/securestring":
		return &awsssm.GetParameterOutput{
			Parameter: &types.Parameter{
				Name:  input.Name,
				Type:  types.ParameterTypeSecureString,
				Value: aws.String("securestring value"),
			},
		}, nil
	}
	return nil, fmt.Errorf("unknown parameter")
}

func TestLookupOk(t *testing.T) {
	tests := []struct {
		testname string
		param    string
		index    []int
		want     string
	}{
		{"String", "/string", nil, "string value"},
		{"StringList 0", "/stringlist", []int{0}, "stringlist value 1"},
		{"StringList 1", "/stringlist", []int{1}, "stringlist value 2"},
		{"SecureString", "/securestring", nil, "securestring value"},
	}
	ctx := context.Background()
	app := newMockApp(mockGetParameter)
	for _, td := range tests {
		t.Run(td.testname, func(t *testing.T) {
			got, err := app.Lookup(ctx, td.param, td.index...)
			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}
			if diff := cmp.Diff(td.want, got); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLookupError(t *testing.T) {
	tests := []struct {
		testname string
		param    string
		index    []int
		err      string
	}{
		{"wrong args count", "/stringlist", []int{0, 1}, "ssm template function accepts at most 2 parameters, but got 3"},
		{"String index", "/string", []int{0}, "the second argument is supported only for StringList type, but the parameter /string is of type String"},
		{"StringList no index", "/stringlist", nil, "the second argument is required for StringList type parrameter to specify the index"},
		{"SecureString index", "/securestring", []int{0}, "the second argument is supported only for StringList type, but the parameter /securestring is of type SecureString"},
	}

	ctx := context.Background()
	app := newMockApp(mockGetParameter)
	for _, td := range tests {
		t.Run(td.testname, func(t *testing.T) {
			_, err := app.Lookup(ctx, td.param, td.index...)
			if diff := cmp.Diff(err.Error(), td.err); diff != "" {
				t.Errorf("got unexpected error %s", diff)
			}
		})
	}
}
