package secretsmanager

import (
	"context"
	"fmt"
	"html/template"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func FuncMap(ctx context.Context, cfg aws.Config) (template.FuncMap, error) {
	smsvc := secretsmanager.NewFromConfig(cfg)
	cache := sync.Map{}
	funcs := template.FuncMap{
		"secretsmanager_arn": func(id string) (string, error) {
			if arn, ok := cache.Load(id); ok {
				return arn.(string), nil
			}
			res, err := smsvc.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
				SecretId: &id,
			})
			if err != nil {
				return "", fmt.Errorf("failed to describe secret: %w", err)
			}
			arn := aws.ToString(res.ARN)
			cache.Store(id, arn)
			return arn, nil
		},
	}
	return funcs, nil
}
