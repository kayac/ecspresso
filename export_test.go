package ecspresso

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
)

var (
	SortTaskDefinitionForDiff    = sortTaskDefinitionForDiff
	SortServiceDefinitionForDiff = sortServiceDefinitionForDiff
	EqualString                  = equalString
	ToNumberCPU                  = toNumberCPU
	ToNumberMemory               = toNumberMemory
	CalcDesiredCount             = calcDesiredCount
	ParseTags                    = parseTags
	ExtractRoleName              = extractRoleName
	IsLongArnFormat              = isLongArnFormat
	ECRImageURLRegex             = ecrImageURLRegex
	NewLogger                    = newLogger
	NewLogFilter                 = newLogFilter
	NewConfigLoader              = newConfigLoader
)

func (d *App) SetLogger(logger *log.Logger) {
	d.logger = logger
}

func SetLogger(logger *log.Logger) {
	commonLogger = logger
}

func (c *Config) SetAWSv2ConfigLoadOptionsFunc(fns []func(*config.LoadOptions) error) {
	c.awsv2ConfigLoadOptionsFunc = fns
}

func (d *App) TaskDefinitionArnForRun(ctx context.Context, opt RunOption) (string, error) {
	return d.taskDefinitionArnForRun(ctx, opt)
}
