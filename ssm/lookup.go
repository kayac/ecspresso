package ssm

import (
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/pkg/errors"
)

// App represents an application
type App struct {
	ssm   ssmiface.SSMAPI
	cache *sync.Map
}

// New creates an application instance
func New(sess *session.Session, cache *sync.Map) *App {
    return &App{
        ssm:   ssm.New(sess),
        cache: cache,
    }
}

// Lookup lookups a parameter from AWS Systems Manager Parameter Store
func (a *App) Lookup(paramName string, index ...int) (outputValue string, err error) {
	if len(index) > 1 {
		return "", errors.Errorf("ssm template function accepts at most 2 parameters, but got %d", len(index)+1)
	}

	param, err := getParameterWithCache(a.ssm, paramName, a.cache)
	if err != nil {
		return "", err
	}

	switch *param.Parameter.Type {
	case ssm.ParameterTypeStringList:
		if len(index) != 1 {
			return "", errors.Errorf("the second argument is required for StringList type parrameter to specify the index")
		}
	case ssm.ParameterTypeString, ssm.ParameterTypeSecureString:
		if len(index) != 0 {
			return "", errors.Errorf("the second argument is supported only for StringList type, but the parameter %s is of type %s", *param.Parameter.Name, *param.Parameter.Type)
		}
	}

	return lookupValue(param, index...)
}

func getParameterWithCache(service ssmiface.SSMAPI, paramName string, cache *sync.Map) (*ssm.GetParameterOutput, error) {
	if cache == nil {
		return getParameter(service, paramName)
	}

	if s, found := cache.Load(paramName); found {
		return s.(*ssm.GetParameterOutput), nil
	}

	if p, err := getParameter(service, paramName); err != nil {
		return nil, err
	} else {
		cache.Store(paramName, p)
		return p, nil
	}
}

func getParameter(service ssmiface.SSMAPI, paramName string) (*ssm.GetParameterOutput, error) {
	res, err := service.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, errors.Wrap(err, "something went wrong calling get-parameter API")
	}
	return res, nil
}

func lookupValue(param *ssm.GetParameterOutput, index ...int) (string, error) {
	switch *param.Parameter.Type {
	case ssm.ParameterTypeString:
		return lookupStringValue(param)
	case ssm.ParameterTypeStringList:
		return lookupStringListValue(param, index...)
	case ssm.ParameterTypeSecureString:
		return lookupSecureString(param)
	}
	return "", errors.Errorf("received unexpected parameter type: %s", *param.Parameter.Type)
}

func lookupStringValue(param *ssm.GetParameterOutput) (string, error) {
	return *param.Parameter.Value, nil
}

func lookupStringListValue(param *ssm.GetParameterOutput, index ...int) (string, error) {
	values := strings.Split(*param.Parameter.Value, ",")
	if len(values) < index[0]-1 {
		return "", errors.Errorf("StringList values were %v, but the index %d is out of range", values, index[0])
	}

	return values[index[0]], nil
}

func lookupSecureString(param *ssm.GetParameterOutput) (string, error) {
	return *param.Parameter.Value, nil
}
