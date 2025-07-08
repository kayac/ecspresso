package secretsmanager

import (
	"context"
	"fmt"
	"html/template"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

type secretsmanagerClient interface {
	DescribeSecret(ctx context.Context, input *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
}

type App struct {
	svc   secretsmanagerClient
	cache *sync.Map
}

func (a *App) ResolveArn(ctx context.Context, id string) (string, error) {
	if arn, ok := a.cache.Load(id); ok {
		return arn.(string), nil
	}
	res, err := a.svc.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: &id,
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe secret: %w", err)
	}
	arn := aws.ToString(res.ARN)
	a.cache.Store(id, arn)
	return arn, nil
}

func NewApp(awsCfg aws.Config) *App {
	return &App{
		svc:   secretsmanager.NewFromConfig(awsCfg),
		cache: &sync.Map{},
	}
}

func FuncMap(ctx context.Context, cfg aws.Config) template.FuncMap {
	app := NewApp(cfg)
	return app.FuncMap(ctx)
}

func JsonnetNativeFuncs(ctx context.Context, cfg aws.Config) ([]*jsonnet.NativeFunction, error) {
	app := NewApp(cfg)
	return app.JsonnetNativeFuncs(ctx), nil
}

func (a *App) FuncMap(ctx context.Context) template.FuncMap {
	funcs := template.FuncMap{
		"secretsmanager_arn": func(id string) (string, error) {
			return a.ResolveArn(ctx, id)
		},
		"secretsmanager_arnf": func(format string, args ...interface{}) (string, error) {
			id := fmt.Sprintf(format, args...)
			return a.ResolveArn(ctx, id)
		},
	}
	return funcs
}

func (a *App) JsonnetNativeFuncs(ctx context.Context) []*jsonnet.NativeFunction {
	return []*jsonnet.NativeFunction{
		{
			Name:   "secretsmanager_arn",
			Params: []ast.Identifier{"id"},
			Func: func(args []any) (any, error) {
				id, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("secretsmanager_arn: id must be string")
				}
				return a.ResolveArn(ctx, id)
			},
		},
		{
			Name:   "secretsmanager_arnf",
			Params: []ast.Identifier{"format", "args"},
			Func: func(args []any) (any, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("secretsmanager_arnf: format argument is required")
				}
				format, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("secretsmanager_arnf: format must be string")
				}
				var formatArgs []interface{}
				if len(args) > 1 {
					formatArgs = args[1:]
				}
				id := fmt.Sprintf(format, formatArgs...)
				return a.ResolveArn(ctx, id)
			},
		},
	}
}
