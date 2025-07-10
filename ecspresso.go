package ecspresso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	aasTypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/aws/aws-sdk-go-v2/service/vpclattice"
	"github.com/aws/smithy-go"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

const DefaultDesiredCount = -1
const DefaultConfigFilePath = "ecspresso.yml"
const dryRunStr = "DRY RUN"

var Version string
var delayForServiceChanged = 3 * time.Second
var waiterMaxDelay = 15 * time.Second
var spcIndent = "  "

type TaskDefinition types.TaskDefinition

func (td *TaskDefinition) Name() string {
	return fmt.Sprintf("%s:%d", aws.ToString(td.Family), td.Revision)
}

type TaskDefinitionInput ecs.RegisterTaskDefinitionInput

func (tdi *TaskDefinitionInput) SetTags(tags []types.Tag) {
	tdi.Tags = tags
}

func (tdi *TaskDefinitionInput) GetTags() []types.Tag {
	return tdi.Tags
}

type Service struct {
	types.Service
	ServiceConnectConfiguration *types.ServiceConnectConfiguration
	VolumeConfigurations        []types.ServiceVolumeConfiguration
	VpcLatticeConfigurations    []types.VpcLatticeConfiguration
	DesiredCount                *int32
}

func (sv *Service) GetTags() []types.Tag {
	return sv.Tags
}

func (sv *Service) SetTags(tags []types.Tag) {
	sv.Tags = tags
}

func (sv *Service) PrimaryDeployment() (types.Deployment, bool) {
	return lo.Find(sv.Deployments, func(dp types.Deployment) bool {
		return aws.ToString(dp.Status) == "PRIMARY"
	})
}

func (d *App) newServiceFromTypes(ctx context.Context, in types.Service) (*Service, error) {
	sv := Service{
		Service:      in,
		DesiredCount: aws.Int32(in.DesiredCount),
	}
	dps := lo.Filter(in.Deployments, func(dp types.Deployment, i int) bool {
		return aws.ToString(dp.Status) == "PRIMARY"
	})
	if sv.isCodeDeploy() {
		// CodeDeploy does not support ServiceConnectConfiguration and VolumeConfigurations
		return &sv, nil
	}
	if len(dps) == 0 {
		d.LogWarn("no primary deployment")
		return &sv, nil
	}
	dp := dps[0]
	d.LogDebug("deployment: %s %s", *dp.Id, *dp.Status)

	// ServiceConnect
	if dp.ServiceConnectConfiguration != nil {
		d.LogDebug("ServiceConnectConfiguration: %#v", dp.ServiceConnectConfiguration)
		sv.ServiceConnectConfiguration = dp.ServiceConnectConfiguration
	}

	// VolumeConfigurations
	if dp.VolumeConfigurations != nil {
		d.LogDebug("VolumeConfigurations: %#v", dp.VolumeConfigurations)
		sv.VolumeConfigurations = dp.VolumeConfigurations
	}

	// VPC Lattice
	if dp.VpcLatticeConfigurations != nil {
		d.LogDebug("VpcLatticeConfigurations: %#v", dp.VpcLatticeConfigurations)
		sv.VpcLatticeConfigurations = dp.VpcLatticeConfigurations
	}

	return &sv, nil
}

func (sv *Service) isCodeDeploy() bool {
	return sv.DeploymentController != nil && sv.DeploymentController.Type == types.DeploymentControllerTypeCodeDeploy
}

type App struct {
	Service string
	Cluster string

	ecs         *ecs.Client
	autoScaling *applicationautoscaling.Client
	codedeploy  *codedeploy.Client
	cwl         *cloudwatchlogs.Client
	iam         *iam.Client
	elbv2       *elasticloadbalancingv2.Client
	sd          *servicediscovery.Client
	lattice     *vpclattice.Client
	verifier    *verifier

	config *Config
	loader *configLoader
	logger *slog.Logger
}

type appOptions struct {
	config *Config
	loader *configLoader
	logger *slog.Logger
}

type AppOption func(*appOptions)

func WithConfig(c *Config) AppOption {
	return func(o *appOptions) {
		o.config = c
	}
}

func WithConfigLoader(extstr, extcode map[string]string) AppOption {
	return func(o *appOptions) {
		o.loader = newConfigLoader(extstr, extcode)
	}
}

func WithLogger(l *slog.Logger) AppOption {
	return func(o *appOptions) {
		o.logger = l
	}
}

func New(ctx context.Context, opt *CLIOptions, newAppOptions ...AppOption) (*App, error) {
	opt.resolveConfigFilePath()

	appOpts := appOptions{
		loader: newConfigLoader(opt.ExtStr, opt.ExtCode),
		logger: newLogger(os.Stderr),
	}
	for _, fn := range newAppOptions {
		fn(&appOpts)
	}

	LogInfo("ecspresso version: %s", Version)

	// load config file
	if appOpts.config == nil {
		if config, err := appOpts.loader.Load(ctx, opt.ConfigFilePath, Version); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", opt.ConfigFilePath, err)
		} else {
			appOpts.config = config
		}
	}
	conf := appOpts.config
	conf.OverrideByCLIOptions(opt)
	conf.AssumeRole(opt.AssumeRoleARN)

	// new app
	d := &App{
		Service: conf.Service,
		Cluster: conf.Cluster,

		ecs:         ecs.NewFromConfig(conf.awsv2Config),
		autoScaling: applicationautoscaling.NewFromConfig(conf.awsv2Config),
		codedeploy:  codedeploy.NewFromConfig(conf.awsv2Config),
		cwl:         cloudwatchlogs.NewFromConfig(conf.awsv2Config),
		iam:         iam.NewFromConfig(conf.awsv2Config),
		elbv2:       elasticloadbalancingv2.NewFromConfig(conf.awsv2Config),
		sd:          servicediscovery.NewFromConfig(conf.awsv2Config),
		lattice:     vpclattice.NewFromConfig(conf.awsv2Config),
		loader:      appOpts.loader,
		config:      appOpts.config,
	}
	if logFormat == logFormatJSON {
		d.logger = appOpts.logger.With("cluster", d.Cluster, "service", d.Service)
	} else {
		d.logger = appOpts.logger.With("", d.Name())
	}

	d.LogDebug("config file path: %s", opt.ConfigFilePath)
	d.LogDebug("timeout: %s", d.config.Timeout)
	return d, nil
}

func (d *App) Config() *Config {
	return d.config
}

func (d *App) Timeout() time.Duration {
	return d.config.Timeout.Duration
}

func (d *App) Start(ctx context.Context) (context.Context, context.CancelFunc) {
	if d.config.Timeout.Duration > 0 {
		return context.WithTimeout(ctx, d.config.Timeout.Duration)
	} else {
		return ctx, func() {}
	}
}

func (d *App) DescribeServicesInput() *ecs.DescribeServicesInput {
	return &ecs.DescribeServicesInput{
		Cluster:  aws.String(d.Cluster),
		Services: []string{d.Service},
		Include:  []types.ServiceField{types.ServiceFieldTags},
	}
}

func (d *App) DescribeTasksInput(task *types.Task) *ecs.DescribeTasksInput {
	return &ecs.DescribeTasksInput{
		Cluster: aws.String(d.Cluster),
		Tasks:   []string{*task.TaskArn},
	}
}

func (d *App) GetLogEventsInput(logGroup string, logStream string, startAt int64, nextToken *string) *cloudwatchlogs.GetLogEventsInput {
	return &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(startAt),
		NextToken:     nextToken,
	}
}

func (d *App) DescribeService(ctx context.Context) (*Service, error) {
	out, err := d.ecs.DescribeServices(ctx, d.DescribeServicesInput())
	if err != nil {
		return nil, fmt.Errorf("failed to describe service: %w", err)
	}
	if len(out.Services) == 0 {
		return nil, ErrNotFound(fmt.Sprintf("service %s is not found", d.Service))
	}
	status := aws.ToString(out.Services[0].Status)
	switch status {
	case "DRAINING":
		d.LogWarn("service %s is %s", d.Service, status)
	case "INACTIVE":
		return nil, ErrNotFound(fmt.Sprintf("service %s is %s", d.Service, status))
	default:
		d.LogDebug("service %s is %s", d.Service, status)
	}
	sv, err := d.newServiceFromTypes(ctx, out.Services[0])
	if err != nil {
		return nil, err
	}
	if err := d.config.Ignore.Apply(sv); err != nil {
		return nil, fmt.Errorf("failed to apply ignore: %w", err)
	}
	return sv, nil
}

type DescribeServiceStatusOutput struct {
	Service        string
	Cluster        string
	TaskDefinition string
	Deployments    []types.Deployment
	TaskSets       []types.TaskSet
	AutoScaling    struct {
		ScalableTargets []aasTypes.ScalableTarget
		ScalingPolicies []aasTypes.ScalingPolicy
	}
	Events []types.ServiceEvent
}

func (s *DescribeServiceStatusOutput) String() string {
	buf := &strings.Builder{}
	fmt.Fprintln(buf, "Service:", s.Service)
	fmt.Fprintln(buf, "Cluster:", arnToName(s.Cluster))
	fmt.Fprintln(buf, "TaskDefinition:", arnToName(s.TaskDefinition))
	if len(s.Deployments) > 0 {
		fmt.Fprintln(buf, "Deployments:")
		for _, dep := range s.Deployments {
			fmt.Fprintln(buf, spcIndent+formatDeployment(dep))
		}
	}
	if len(s.TaskSets) > 0 {
		fmt.Fprintln(buf, "TaskSets:")
		for _, ts := range s.TaskSets {
			fmt.Fprintln(buf, spcIndent+formatTaskSet(ts))
		}
	}

	fmt.Fprintln(buf, "AutoScaling:")
	for _, target := range s.AutoScaling.ScalableTargets {
		fmt.Fprintln(buf, formatScalableTarget(target))
	}
	for _, policy := range s.AutoScaling.ScalingPolicies {
		fmt.Fprintln(buf, formatScalingPolicy(policy))
	}

	fmt.Fprintln(buf, "Events:")
	for _, e := range s.Events {
		fmt.Fprintln(buf, serviceEvent(e).String())
	}
	return buf.String()
}

func (d *App) DescribeServiceStatus(ctx context.Context, events int) (*Service, error) {
	s, err := d.DescribeService(ctx)
	if err != nil {
		return nil, err
	}
	out := &DescribeServiceStatusOutput{
		Service:        *s.ServiceName,
		Cluster:        *s.ClusterArn,
		TaskDefinition: *s.TaskDefinition,
		Deployments:    s.Deployments,
		TaskSets:       s.TaskSets,
	}

	sort.SliceStable(s.Events, func(i, j int) bool {
		return s.Events[i].CreatedAt.Before(*s.Events[j].CreatedAt)
	})
	head := lo.Max([]int{len(s.Events) - events, 0})
	for i := head; i < len(s.Events); i++ {
		out.Events = append(out.Events, s.Events[i])
	}

	out.AutoScaling.ScalableTargets, out.AutoScaling.ScalingPolicies, err = d.describeAutoScaling(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("failed to describe autoscaling: %w", err)
	}

	if _, err := WriteOutput(out); err != nil {
		return nil, fmt.Errorf("failed to write output: %w", err)
	}
	return s, nil
}

func (d *App) describeAutoScaling(ctx context.Context, s *Service) ([]aasTypes.ScalableTarget, []aasTypes.ScalingPolicy, error) {
	resourceId := fmt.Sprintf("service/%s/%s", arnToName(*s.ClusterArn), *s.ServiceName)
	tout, err := d.autoScaling.DescribeScalableTargets(
		ctx,
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []string{resourceId},
			ServiceNamespace:  aasTypes.ServiceNamespaceEcs,
			ScalableDimension: aasTypes.ScalableDimensionECSServiceDesiredCount,
		},
	)
	if err != nil {
		var oe *smithy.OperationError
		if errors.As(err, &oe) {
			d.LogWarn("failed to describe scalable targets: %s", oe)
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to describe scalable targets: %w", err)
	}
	if len(tout.ScalableTargets) == 0 {
		return tout.ScalableTargets, nil, nil
	}

	pout, err := d.autoScaling.DescribeScalingPolicies(
		ctx,
		&applicationautoscaling.DescribeScalingPoliciesInput{
			ResourceId:        &resourceId,
			ServiceNamespace:  aasTypes.ServiceNamespaceEcs,
			ScalableDimension: aasTypes.ScalableDimensionECSServiceDesiredCount,
		},
	)
	if err != nil {
		var oe *smithy.OperationError
		if errors.As(err, &oe) {
			d.LogWarn("failed to describe scaling policies: %s", oe)
			return tout.ScalableTargets, nil, nil
		}
		return tout.ScalableTargets, nil, fmt.Errorf("failed to describe scaling policies: %w", err)
	}

	return tout.ScalableTargets, pout.ScalingPolicies, nil
}

func (d *App) DescribeTaskStatus(ctx context.Context, task *types.Task, watchContainer *types.ContainerDefinition) error {
	out, err := d.ecs.DescribeTasks(ctx, d.DescribeTasksInput(task))
	if err != nil {
		return fmt.Errorf("failed to describe tasks: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.LogInfo("Task ARN: " + *f.Arn)
		return fmt.Errorf(*f.Reason)
	}

	ts := out.Tasks[0]
	if ts.StopCode == types.TaskStopCodeTaskFailedToStart {
		return fmt.Errorf("task failed to start: %s", aws.ToString(ts.StoppedReason))
	}

	var container *types.Container
	for _, c := range ts.Containers {
		if *c.Name == *watchContainer.Name {
			container = &c
			break
		}
	}
	if container == nil {
		container = &(ts.Containers[0])
	}

	if container.ExitCode != nil && *container.ExitCode != 0 {
		msg := fmt.Sprintf("container: %s, exit code: %s", *container.Name, strconv.FormatInt(int64(*container.ExitCode), 10))
		if container.Reason != nil {
			msg += ", reason: " + *container.Reason
		}
		return errors.New(msg)
	} else if container.Reason != nil {
		return fmt.Errorf("container: %s, reason: %s", *container.Name, *container.Reason)
	}
	return nil
}

func (d *App) DescribeTaskDefinition(ctx context.Context, tdArn string) (*TaskDefinitionInput, error) {
	out, err := d.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &tdArn,
		Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}
	td := TaskDefinition(*out.TaskDefinition)
	tdi := tdToTaskDefinitionInput(&td, out.Tags)
	if err := d.config.Ignore.Apply(tdi); err != nil {
		return nil, fmt.Errorf("failed to apply ignore: %w", err)
	}
	return tdi, nil
}

func (d *App) GetLogEvents(ctx context.Context, logGroup string, logStream string, startedAt time.Time, nextToken *string) (*string, error) {
	ms := startedAt.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	out, err := d.cwl.GetLogEvents(ctx, d.GetLogEventsInput(logGroup, logStream, ms, nextToken))
	if err != nil {
		return nextToken, err
	}
	if len(out.Events) == 0 {
		return nextToken, nil
	}
	for _, event := range out.Events {
		WriteOutput(logEvent(event))
	}
	return out.NextForwardToken, nil
}

func containerOf(td *TaskDefinitionInput, name *string) *types.ContainerDefinition {
	if name == nil || *name == "" {
		return &td.ContainerDefinitions[0]
	}
	for _, c := range td.ContainerDefinitions {
		if *c.Name == *name {
			c := c
			return &c
		}
	}
	return nil
}

func (d *App) findLatestTaskDefinitionArn(ctx context.Context, family string) (string, error) {
	out, err := d.ecs.ListTaskDefinitions(ctx,
		&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String(family),
			MaxResults:   aws.Int32(1),
			Sort:         types.SortOrderDesc,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list task definitions: %w", err)
	}
	if len(out.TaskDefinitionArns) == 0 {
		return "", ErrNotFound(fmt.Sprintf("no task definitions family %s are found", family))
	}
	return out.TaskDefinitionArns[0], nil
}

func (d *App) Name() string {
	return fmt.Sprintf("%s/%s", d.Service, d.Cluster)
}

func (d *App) RegisterTaskDefinition(ctx context.Context, td *TaskDefinitionInput) (*TaskDefinition, error) {
	d.LogInfo("Registering a new task definition...")
	if len(td.Tags) == 0 {
		td.Tags = nil // Tags can not be empty.
	}
	tdi := ecs.RegisterTaskDefinitionInput(*td)
	out, err := d.ecs.RegisterTaskDefinition(
		ctx,
		&tdi,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register task definition: %w", err)
	}
	otd := TaskDefinition(*out.TaskDefinition)
	d.LogInfo("Task definition is registered %s", otd.Name())
	return &otd, nil
}

func (d *App) LoadTaskDefinition(path string) (*TaskDefinitionInput, error) {
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load task definition %s: %w", path, err)
	}
	c := struct {
		TaskDefinition json.RawMessage `json:"taskDefinition"`
	}{}
	dec := json.NewDecoder(bytes.NewReader(src))
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("failed to load task definition %s: %w", path, err)
	}
	if c.TaskDefinition != nil {
		src = c.TaskDefinition
	}
	var td TaskDefinitionInput
	if err := UnmarshalJSONForStruct(src, &td, path); err != nil {
		return nil, fmt.Errorf("failed to load task definition %s: %w", path, err)
	}
	if len(td.Tags) == 0 {
		td.Tags = nil
	}
	if err := d.config.Ignore.Apply(&td); err != nil {
		return nil, fmt.Errorf("failed to apply ignore: %w", err)
	}

	return &td, nil
}

func unmarshalYAML(src []byte, v any, path string) error {
	b, err := yaml.YAMLToJSON(src)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return unmarshalJSON(b, v, path)
}

func unmarshalJSON(src []byte, v any, path string) error {
	strict := json.NewDecoder(bytes.NewReader(src))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&v); err != nil {
		if !strings.Contains(err.Error(), "unknown field") {
			return err
		}
		LogWarn("%s in %s", err, path)
		// unknown field -> try lax decoder
		lax := json.NewDecoder(bytes.NewReader(src))
		return lax.Decode(&v)
	}
	return nil
}

func (d *App) LoadServiceDefinition(path string) (*Service, error) {
	if path == "" {
		return nil, fmt.Errorf("service_definition is not defined")
	}

	var sv Service
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load service definition %s: %w", path, err)
	}
	if err := unmarshalJSON(src, &sv, path); err != nil {
		return nil, fmt.Errorf("failed to load service definition %s: %w", path, err)
	}

	sv.ServiceName = aws.String(d.config.Service)
	if sv.DesiredCount == nil {
		d.LogDebug("Loaded DesiredCount: nil (-1)")
	} else {
		d.LogDebug("Loaded DesiredCount: %d", *sv.DesiredCount)
	}

	if err := d.config.Ignore.Apply(&sv); err != nil {
		return nil, fmt.Errorf("failed to apply ignore: %w", err)
	}
	return &sv, nil
}

func (d *App) GetLogInfo(task *types.Task, c *types.ContainerDefinition) (string, string) {
	p := strings.Split(*task.TaskArn, "/")
	taskID := p[len(p)-1]
	lc := c.LogConfiguration
	logStreamPrefix := lc.Options["awslogs-stream-prefix"]

	logStream := strings.Join([]string{logStreamPrefix, *c.Name, taskID}, "/")
	logGroup := lc.Options["awslogs-group"]

	d.LogInfo("logGroup: %s", logGroup)
	d.LogInfo("logStream: %s", logStream)

	return logGroup, logStream
}

func (d *App) FilterCommand() string {
	return d.config.FilterCommand
}
