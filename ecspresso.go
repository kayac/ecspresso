package ecspresso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

type TaskDefinition = types.TaskDefinition

type TaskDefinitionInput = ecs.RegisterTaskDefinitionInput

func taskDefinitionName(t *TaskDefinition) string {
	return fmt.Sprintf("%s:%d", *t.Family, t.Revision)
}

type Service struct {
	types.Service
	ServiceConnectConfiguration *types.ServiceConnectConfiguration
	VolumeConfigurations        []types.ServiceVolumeConfiguration
	DesiredCount                *int32
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
		d.Log("[WARNING] no primary deployment")
		return &sv, nil
	}
	dp := dps[0]
	d.Log("[DEBUG] deployment: %s %s", *dp.Id, *dp.Status)

	// ServiceConnect
	scc := dp.ServiceConnectConfiguration
	sv.ServiceConnectConfiguration = scc
	if scc != nil {
		// resolve sd namespace arn to name
		id := arnToName(aws.ToString(scc.Namespace))
		res, err := d.sd.GetNamespace(ctx, &servicediscovery.GetNamespaceInput{
			Id: &id,
		})
		if err != nil {
			var oe *smithy.OperationError
			if errors.As(err, &oe) {
				d.Log("[WARNING] failed to get namespace: %s", oe)
			} else {
				return nil, fmt.Errorf("failed to get namespace: %w", err)
			}
		}
		if res.Namespace != nil {
			scc.Namespace = res.Namespace.Name
		}
	}

	// VolumeConfigurations
	if dp.VolumeConfigurations != nil {
		d.Log("[DEBUG] VolumeConfigurations: %#v", dp.VolumeConfigurations)
		sv.VolumeConfigurations = dp.VolumeConfigurations
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
	verifier    *verifier

	config *Config
	loader *configLoader
	logger *log.Logger
}

type appOptions struct {
	config *Config
	loader *configLoader
	logger *log.Logger
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

func WithLogger(l *log.Logger) AppOption {
	return func(o *appOptions) {
		o.logger = l
	}
}

func New(ctx context.Context, opt *CLIOptions, newAppOptions ...AppOption) (*App, error) {
	opt.resolveConfigFilePath()

	appOpts := appOptions{
		loader: newConfigLoader(opt.ExtStr, opt.ExtCode),
		logger: newLogger(),
	}
	for _, fn := range newAppOptions {
		fn(&appOpts)
	}

	// set log level
	if opt.Debug {
		appOpts.logger.SetOutput(newLogFilter(os.Stderr, "DEBUG"))
	} else {
		appOpts.logger.SetOutput(newLogFilter(os.Stderr, "INFO"))
	}
	Log("[INFO] ecspresso version: %s", Version)

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
		loader:      appOpts.loader,
		config:      appOpts.config,
		logger:      appOpts.logger,
	}

	d.Log("[DEBUG] config file path: %s", opt.ConfigFilePath)
	d.Log("[DEBUG] timeout: %s", d.config.Timeout)
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
		d.Log("[WARNING] service %s is %s", d.Service, status)
	case "INACTIVE":
		return nil, ErrNotFound(fmt.Sprintf("service %s is %s", d.Service, status))
	default:
		d.Log("[DEBUG] service %s is %s", d.Service, status)
	}
	return d.newServiceFromTypes(ctx, out.Services[0])
}

func (d *App) DescribeServiceStatus(ctx context.Context, events int) (*Service, error) {
	s, err := d.DescribeService(ctx)
	if err != nil {
		return nil, err
	}
	fmt.Println("Service:", *s.ServiceName)
	fmt.Println("Cluster:", arnToName(*s.ClusterArn))
	fmt.Println("TaskDefinition:", arnToName(*s.TaskDefinition))
	if len(s.Deployments) > 0 {
		fmt.Println("Deployments:")
		for _, dep := range s.Deployments {
			fmt.Println(spcIndent + formatDeployment(dep))
		}
	}
	if len(s.TaskSets) > 0 {
		fmt.Println("TaskSets:")
		for _, ts := range s.TaskSets {
			fmt.Println(spcIndent + formatTaskSet(ts))
		}
	}

	if err := d.describeAutoScaling(ctx, s); err != nil {
		return nil, fmt.Errorf("failed to describe autoscaling: %w", err)
	}

	fmt.Println("Events:")
	sort.SliceStable(s.Events, func(i, j int) bool {
		return s.Events[i].CreatedAt.Before(*s.Events[j].CreatedAt)
	})
	head := lo.Max([]int{len(s.Events) - events, 0})
	for i := head; i < len(s.Events); i++ {
		fmt.Println(formatEvent(s.Events[i]))
	}
	return s, nil
}

func (d *App) describeAutoScaling(ctx context.Context, s *Service) error {
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
			d.Log("[WARNING] failed to describe scalable targets: %s", oe)
			return nil
		}
		return fmt.Errorf("failed to describe scalable targets: %w", err)
	}
	if len(tout.ScalableTargets) == 0 {
		return nil
	}

	fmt.Println("AutoScaling:")
	for _, target := range tout.ScalableTargets {
		fmt.Println(formatScalableTarget(target))
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
			d.Log("[WARNING] failed to describe scaling policies: %s", oe)
			return nil
		}
		return fmt.Errorf("failed to describe scaling policies: %w", err)
	}
	for _, policy := range pout.ScalingPolicies {
		fmt.Println(formatScalingPolicy(policy))
	}
	return nil
}

func (d *App) DescribeTaskStatus(ctx context.Context, task *types.Task, watchContainer *types.ContainerDefinition) error {
	out, err := d.ecs.DescribeTasks(ctx, d.DescribeTasksInput(task))
	if err != nil {
		return fmt.Errorf("failed to describe tasks: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.Log("Task ARN: " + *f.Arn)
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
		return fmt.Errorf(msg)
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
	return tdToTaskDefinitionInput(out.TaskDefinition, out.Tags), nil
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
		fmt.Println(formatLogEvent(event))
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
		return "", fmt.Errorf("failed to list taskdefinitions: %w", err)
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
	d.Log("Registering a new task definition...")
	if len(td.Tags) == 0 {
		td.Tags = nil // Tags can not be empty.
	}
	out, err := d.ecs.RegisterTaskDefinition(
		ctx,
		td,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register task definition: %w", err)
	}
	d.Log("Task definition is registered %s", taskDefinitionName(out.TaskDefinition))
	return out.TaskDefinition, nil
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
	return &td, nil
}

func unmarshalYAML(src []byte, v interface{}, path string) error {
	b, err := yaml.YAMLToJSON(src)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return unmarshalJSON(b, v, path)
}

func unmarshalJSON(src []byte, v interface{}, path string) error {
	strict := json.NewDecoder(bytes.NewReader(src))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&v); err != nil {
		if !strings.Contains(err.Error(), "unknown field") {
			return err
		}
		Log("[WARNING] %s in %s", err, path)
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
		d.Log("[DEBUG] Loaded DesiredCount: nil (-1)")
	} else {
		d.Log("[DEBUG] Loaded DesiredCount: %d", *sv.DesiredCount)
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

	d.Log("logGroup: %s", logGroup)
	d.Log("logStream: %s", logStream)

	return logGroup, logStream
}

func (d *App) FilterCommand() string {
	return d.config.FilterCommand
}
