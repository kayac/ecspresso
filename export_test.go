package ecspresso

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

var (
	SortTaskDefinition     = sortTaskDefinition
	ToNumberCPU            = toNumberCPU
	ToNumberMemory         = toNumberMemory
	CalcDesiredCount       = calcDesiredCount
	ParseTags              = parseTags
	ParseImageURL          = parseImageURL
	ExtractRoleName        = extractRoleName
	IsLongArnFormat        = isLongArnFormat
	ECRImageURLRegex       = ecrImageURLRegex
	NewLogger              = newLogger
	NewConfigLoader        = newConfigLoader
	LogLevel               = logLevel
	SetLogFormat           = setLogFormat
	NewVerifier            = newVerifier
	ArnToName              = arnToName
	NewVerifyState         = newVerifyState
	Map2str                = map2str
	DiffServices           = diffServices
	DiffTaskDefs           = diffTaskDefs
	IsPermissionError      = isPermissionError
	WrapPermissionError    = wrapPermissionError
	ParseIAMPolicyDocument = parseIAMPolicyDocument
	ParseSections          = parseSections
	ReadmeContent          = readmeContent
)

type ModifyAutoScalingParams = modifyAutoScalingParams
type IAMPolicyDocument = iamPolicyDocument
type StringOrSlice = stringOrSlice

func (s StringOrSlice) Contains(v string) bool {
	return s.contains(v)
}

func (d *App) SetLogger(logger *slog.Logger) {
	d.logger = logger
}

func SetLogger(logger *slog.Logger) {
	commonLogger = logger
}

func SetAWSV2ConfigLoadOptionsFunc(f []func(*config.LoadOptions) error) {
	awsv2ConfigLoadOptionsFunc = f
}

func ResetAWSV2ConfigLoadOptionsFunc() {
	awsv2ConfigLoadOptionsFunc = nil
}

type TaskDefinitionForRun = taskDefinitionForRun

func (d *App) ResolveTaskDefinitionForRun(ctx context.Context, opt RunOption) (*taskDefinitionForRun, error) {
	return d.resolveTaskDefinitionForRun(ctx, opt)
}

func (opt *DiffOption) SetWriter(w io.Writer) {
	opt.w = w
}

func (i *ConfigIgnore) FilterTags(tags []types.Tag) []types.Tag {
	return i.filterTags(tags)
}

// SleepContext exposes sleepContext for testing
func SleepContext(ctx context.Context, d time.Duration) {
	sleepContext(ctx, d)
}

func (opts *CLIOptions) ForSubCommand(sub string) any {
	return opts.forSubCommand(sub)
}
