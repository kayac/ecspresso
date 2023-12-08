package secretsmanager

import (
	"context"
	"fmt"
	"html/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func FuncMap(ctx context.Context, cfg aws.Config) (template.FuncMap, error) {
	smsvc := secretsmanager.NewFromConfig(cfg)
	funcs := template.FuncMap{
		"secretsmanager_arn": func(id string) (string, error) {
			res, err := smsvc.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
				SecretId: &id,
			})
			if err != nil {
				return "", fmt.Errorf("failed to describe secret: %w", err)
			}
			return *res.ARN, nil
		},
	}
	return funcs, nil
}
