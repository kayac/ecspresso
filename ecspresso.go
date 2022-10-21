package ecspresso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	"github.com/mattn/go-isatty"
)

const DefaultDesiredCount = -1
const dryRunStr = "DRY RUN"

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())
var TerminalWidth = 90
var delayForServiceChanged = 3 * time.Second
var spcIndent = "  "

type TaskDefinition = types.TaskDefinition

type TaskDefinitionInput = ecs.RegisterTaskDefinitionInput

func taskDefinitionName(t *TaskDefinition) string {
	return fmt.Sprintf("%s:%d", *t.Family, t.Revision)
}

type Service struct {
	types.Service
	DesiredCount *int32
}

func newServiceFromTypes(sv types.Service) *Service {
	return &Service{
		Service:      sv,
		DesiredCount: aws.Int32(sv.DesiredCount),
	}
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
	verifier    *verifier

	config *Config
	loader *configLoader
	logger *log.Logger
}

func New(ctx context.Context, opt *Option) (*App, error) {
	loader := newConfigLoader(opt.ExtStr, opt.ExtCode)
	var (
		conf *Config
		err  error
	)
	if opt.InitOption != nil {
		conf, err = opt.InitOption.NewConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize config: %w", err)
		}
	} else {
		conf, err = loader.Load(ctx, opt.ConfigFilePath, opt.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", opt.ConfigFilePath, err)
		}
	}

	logger := newLogger()
	if opt.Debug {
		logger.SetOutput(newLogFilter(os.Stderr, "DEBUG"))
	} else {
		logger.SetOutput(newLogFilter(os.Stderr, "INFO"))
	}
	d := &App{
		Service: conf.Service,
		Cluster: conf.Cluster,

		ecs:         ecs.NewFromConfig(conf.awsv2Config),
		autoScaling: applicationautoscaling.NewFromConfig(conf.awsv2Config),
		codedeploy:  codedeploy.NewFromConfig(conf.awsv2Config),
		cwl:         cloudwatchlogs.NewFromConfig(conf.awsv2Config),
		iam:         iam.NewFromConfig(conf.awsv2Config),
		elbv2:       elasticloadbalancingv2.NewFromConfig(conf.awsv2Config),

		config: conf,
		loader: loader,
		logger: logger,
	}
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

type Option struct {
	InitOption     *InitOption
	ConfigFilePath string
	Version        string
	Debug          bool
	ExtStr         map[string]string
	ExtCode        map[string]string
}

func (d *App) DescribeServicesInput() *ecs.DescribeServicesInput {
	return &ecs.DescribeServicesInput{
		Cluster:  aws.String(d.Cluster),
		Services: []string{d.Service},
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
	if s := aws.ToString(out.Services[0].Status); s == "INACTIVE" {
		return nil, ErrNotFound(fmt.Sprintf("service %s is %s", d.Service, s))
	}
	return newServiceFromTypes(out.Services[0]), nil
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
	for i, event := range s.Events {
		if i >= events {
			break
		}
		for _, line := range formatEvent(event, TerminalWidth) {
			fmt.Println(line)
		}
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
		return fmt.Errorf("failed to describe scaling policies: %w", err)
	}
	for _, policy := range pout.ScalingPolicies {
		fmt.Println(formatScalingPolicy(policy))
	}
	return nil
}

func (d *App) DescribeServiceDeployments(ctx context.Context, startedAt time.Time) (int, error) {
	out, err := d.ecs.DescribeServices(ctx, d.DescribeServicesInput())
	if err != nil {
		return 0, fmt.Errorf("failed to describe service deployments: %w", err)
	}
	if len(out.Services) == 0 {
		return 0, nil
	}
	s := out.Services[0]
	lines := 0
	for _, dep := range s.Deployments {
		lines++
		d.Log(formatDeployment(dep))
	}
	for _, event := range s.Events {
		if (*event.CreatedAt).After(startedAt) {
			for _, line := range formatEvent(event, TerminalWidth) {
				fmt.Println(line)
				lines++
			}
		}
	}
	return lines, nil
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

	var container *types.Container
	for _, c := range out.Tasks[0].Containers {
		if *c.Name == *watchContainer.Name {
			container = &c
			break
		}
	}
	if container == nil {
		container = &(out.Tasks[0].Containers[0])
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
		for _, line := range formatLogEvent(event, TerminalWidth) {
			fmt.Println(line)
		}
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
	if err := d.UnmarshalJSONForStruct(src, &td, path); err != nil {
		return nil, fmt.Errorf("failed to load task definition %s: %w", path, err)
	}
	if len(td.Tags) == 0 {
		td.Tags = nil
	}
	return &td, nil
}

func (d *App) unmarshalJSON(src []byte, v interface{}, path string) error {
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
	if err := d.unmarshalJSON(src, &sv, path); err != nil {
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

func (d *App) suspendAutoScaling(ctx context.Context, suspendState bool) error {
	resourceId := fmt.Sprintf("service/%s/%s", d.Cluster, d.Service)

	out, err := d.autoScaling.DescribeScalableTargets(
		ctx,
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []string{resourceId},
			ServiceNamespace:  aasTypes.ServiceNamespaceEcs,
			ScalableDimension: aasTypes.ScalableDimensionECSServiceDesiredCount,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to describe scalable targets: %w", err)
	}
	if len(out.ScalableTargets) == 0 {
		d.Log("No scalable target for %s", resourceId)
		return nil
	}
	for _, target := range out.ScalableTargets {
		d.Log("Register scalable target %s set suspend state to %t", *target.ResourceId, suspendState)
		_, err := d.autoScaling.RegisterScalableTarget(
			ctx,
			&applicationautoscaling.RegisterScalableTargetInput{
				ServiceNamespace:  target.ServiceNamespace,
				ScalableDimension: target.ScalableDimension,
				ResourceId:        target.ResourceId,
				SuspendedState: &aasTypes.SuspendedState{
					DynamicScalingInSuspended:  &suspendState,
					DynamicScalingOutSuspended: &suspendState,
					ScheduledScalingSuspended:  &suspendState,
				},
			},
		)
		if err != nil {
			return fmt.Errorf("failed to register scalable target %s set suspend state to %t: %w", *target.ResourceId, suspendState, err)
		}
	}
	return nil
}
