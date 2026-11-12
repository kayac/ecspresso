package ecspresso

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	logsTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

var (
	UnmarshalJSON          = unmarshalJSON
	UnwrapTaskDefinition   = unwrapTaskDefinition
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
	ExtractHookDetails     = extractHookDetails
	IsPermissionError      = isPermissionError
	WrapPermissionError    = wrapPermissionError
	ParseIAMPolicyDocument = parseIAMPolicyDocument
	ParseSections          = parseSections
	ReadmeContent          = readmeContent
	GetArticle             = getArticle

	LifecycleStageIndex                  = lifecycleStageIndex
	ValidateLifecycleStageSupported      = validateLifecycleStageSupported
	EvaluateDeploymentStatus             = evaluateDeploymentStatus
	PausedHookIDs                        = pausedHookIDs
	EarlySuccessCriteriaOf               = earlySuccessCriteriaOf
	EarlySuccessCriteriaCompletedMessage = earlySuccessCriteriaCompletedMessage
)

func (d *App) WarnEarlySuccessCriteriaWait(sv *Service, until waitUntil) bool {
	return d.warnEarlySuccessCriteriaWait(sv, until)
}

func (sv *Service) PrimaryDeploymentStartedAfter(t time.Time) bool {
	return sv.primaryDeploymentStartedAfter(t)
}

func (sv *Service) EarlySuccessCriteriaEnabled() bool {
	return sv.earlySuccessCriteriaEnabled()
}

type WaitDeploymentResult = waitDeploymentResult
type WaitUntil = waitUntil

const (
	WaitDeploymentContinue  = waitDeploymentContinue
	WaitDeploymentCompleted = waitDeploymentCompleted
	WaitDeploymentDone      = waitDeploymentDone
)

func (d *App) DeploymentPaused(dp *types.ServiceDeployment) bool {
	return d.deploymentPaused(dp)
}

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

func (d *App) VerifyDeploymentConfiguration(ctx context.Context, dc *types.DeploymentConfiguration) error {
	return d.verifyDeploymentConfiguration(ctx, dc)
}

func ContextWithVerifyState(ctx context.Context, vs *verifyState) context.Context {
	return context.WithValue(ctx, verifyStateKey, vs)
}

var (
	DecodeAuthorizationToken   = decodeAuthorizationToken
	WithECRPublicAnonymousHint = withECRPublicAnonymousHint
)

func (v *verifier) ECRPublicCredentials(ctx context.Context) (string, string, error) {
	return v.ecrPublicCredentials(ctx)
}

// FormatLogEvent returns the text and JSON representations of a task log event.
func FormatLogEvent(event logsTypes.OutputLogEvent, taskID string) (string, string) {
	e := logEvent{event: event, taskID: taskID}
	return e.String(), e.JSON()
}

// SetTaskPollInterval overrides the task polling interval until the test ends.
func SetTaskPollInterval(t interface{ Cleanup(func()) }, d time.Duration) {
	orig := taskPollInterval
	taskPollInterval = d
	t.Cleanup(func() { taskPollInterval = orig })
}
