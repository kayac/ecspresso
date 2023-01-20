package ecspresso_test

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var middlewareResults = map[string]interface{}{
	"DescribeServices": &ecs.DescribeServicesOutput{
		Services: []types.Service{
			{
				TaskDefinition: ptr("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:39"),
			},
		},
	},
	"ListTaskDefinitions": &ecs.ListTaskDefinitionsOutput{
		TaskDefinitionArns: []string{
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:45",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:44",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:43",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:42",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:41",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:40",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:39",
			"arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:38",
		},
	},
}

func SDKTestingMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc(
				"test",
				func(ctx context.Context, in middleware.FinalizeInput, handler middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req := in.Request.(*smithyhttp.Request)
					target := strings.SplitN(req.Header.Get("X-Amz-Target"), ".", 2)[1]
					return middleware.FinalizeOutput{
						Result: middlewareResults[target],
					}, middleware.Metadata{}, nil
				},
			),
			middleware.Before,
		)
	}
}
