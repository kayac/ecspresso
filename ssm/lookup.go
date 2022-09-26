package ssm

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// App represents an application
type App struct {
	ssm   ssmiface
	cache *sync.Map
}

type ssmiface interface {
	GetParameter(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// New creates an application instance
func New(cfg aws.Config, cache *sync.Map) *App {
	return &App{
		ssm:   ssm.NewFromConfig(cfg),
		cache: cache,
	}
}

// Lookup lookups a parameter from AWS Systems Manager Parameter Store
func (a *App) Lookup(ctx context.Context, paramName string, index ...int) (outputValue string, err error) {
	if len(index) > 1 {
		return "", fmt.Errorf("ssm template function accepts at most 2 parameters, but got %d", len(index)+1)
	}

	param, err := getParameterWithCache(ctx, a.ssm, paramName, a.cache)
	if err != nil {
		return "", err
	}

	switch param.Parameter.Type {
	case types.ParameterTypeStringList:
		if len(index) != 1 {
			return "", fmt.Errorf("the second argument is required for StringList type parrameter to specify the index")
		}
	case types.ParameterTypeString, types.ParameterTypeSecureString:
		if len(index) != 0 {
			return "", fmt.Errorf("the second argument is supported only for StringList type, but the parameter %s is of type %s", *param.Parameter.Name, param.Parameter.Type)
		}
	}

	return lookupValue(param, index...)
}

func getParameterWithCache(ctx context.Context, service ssmiface, paramName string, cache *sync.Map) (*ssm.GetParameterOutput, error) {
	if cache == nil {
		return getParameter(ctx, service, paramName)
	}

	if s, found := cache.Load(paramName); found {
		return s.(*ssm.GetParameterOutput), nil
	}

	if p, err := getParameter(ctx, service, paramName); err != nil {
		return nil, err
	} else {
		cache.Store(paramName, p)
		return p, nil
	}
}

func getParameter(ctx context.Context, service ssmiface, paramName string) (*ssm.GetParameterOutput, error) {
	res, err := service.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("something went wrong calling get-parameter API: %w", err)
	}
	return res, nil
}

func lookupValue(param *ssm.GetParameterOutput, index ...int) (string, error) {
	switch param.Parameter.Type {
	case types.ParameterTypeString:
		return lookupStringValue(param)
	case types.ParameterTypeStringList:
		return lookupStringListValue(param, index...)
	case types.ParameterTypeSecureString:
		return lookupSecureString(param)
	}
	return "", fmt.Errorf("received unexpected parameter type: %s", param.Parameter.Type)
}

func lookupStringValue(param *ssm.GetParameterOutput) (string, error) {
	return *param.Parameter.Value, nil
}

func lookupStringListValue(param *ssm.GetParameterOutput, index ...int) (string, error) {
	values := strings.Split(*param.Parameter.Value, ",")
	if len(values) < index[0]-1 {
		return "", fmt.Errorf("StringList values were %v, but the index %d is out of range", values, index[0])
	}

	return values[index[0]], nil
}

func lookupSecureString(param *ssm.GetParameterOutput) (string, error) {
	return *param.Parameter.Value, nil
}
