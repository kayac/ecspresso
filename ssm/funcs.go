package ssm

import (
	"fmt"
	"sync"
	"text/template"

	"github.com/aws/aws-sdk-go/aws/session"
)

func FuncMap(sess *session.Session) (template.FuncMap, error) {
	cache := sync.Map{}
	app := New(sess, &cache)

	return template.FuncMap{
		"ssm": func(paramName string, index ...int) (string, error) {
			value, err := app.Lookup(paramName, index...)
			if err != nil {
				return "", fmt.Errorf("failed to lookup ssm parameter: %w", err)
			}
			return value, nil
		},
	}, nil
}
