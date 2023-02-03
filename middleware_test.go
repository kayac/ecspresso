package ecspresso_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var middlewareResults = map[string]func(string) any{
	"DescribeServices": func(family string) any {
		return &ecs.DescribeServicesOutput{
			Services: []types.Service{
				{
					TaskDefinition: ptr(fmt.Sprintf("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/%s:39", family)),
				},
			},
		}
	},
	"ListTaskDefinitions": func(family string) any {
		td := func(rev int) string {
			return fmt.Sprintf("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/%s:%d", family, rev)
		}
		return &ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []string{
				td(45), td(44), td(43), td(42), td(41), td(40), td(39), td(38), td(37), td(36),
			},
		}
	},
}

func SDKTestingMiddleware(family string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc(
				"test",
				func(ctx context.Context, in middleware.FinalizeInput, handler middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req := in.Request.(*smithyhttp.Request)
					target := strings.SplitN(req.Header.Get("X-Amz-Target"), ".", 2)[1]
					return middleware.FinalizeOutput{
						Result: middlewareResults[target](family),
					}, middleware.Metadata{}, nil
				},
			),
			middleware.Before,
		)
	}
}
