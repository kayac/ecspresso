package cloudformation

import (
	"sync"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/pkg/errors"
)

var mu sync.Mutex
var cache = make(map[string]*cloudformation.Stack, 1)

func lookup(cfn *cloudformation.CloudFormation, stackName, outputKey string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	stack := cache[stackName]
	if stack == nil {
		out, err := cfn.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})
		if err != nil {
			return "", errors.Wrapf(err, "failed to describe stacks: %s", stackName)
		}
		if len(out.Stacks) == 0 {
			return "", errors.Wrapf(err, "no such stack: %s", stackName)
		}
		stack = out.Stacks[0]
		cache[stackName] = out.Stacks[0]
	}
	return lookupStackOutput(stack, outputKey)
}

func lookupStackOutput(stack *cloudformation.Stack, outputKey string) (string, error) {
	for _, outputs := range stack.Outputs {
		if aws.StringValue(outputs.OutputKey) == outputKey {
			return aws.StringValue(outputs.OutputValue), nil
		}
	}
	return "", errors.Errorf("OutputKey %s is not found in stack %s", outputKey, *stack.StackName)
}

func NewFuncs(sess *session.Session) template.FuncMap {
	cfn := cloudformation.New(sess)
	return template.FuncMap{
		"cfn_output": func(stackName, outputKey string) string {
			outputValue, err := lookup(cfn, stackName, outputKey)
			if err != nil {
				panic(err)
			}
			return outputValue
		},
	}
}
