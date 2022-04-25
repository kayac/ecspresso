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

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/fatih/color"
	gc "github.com/kayac/go-config"
	"github.com/mattn/go-isatty"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
)

const DefaultDesiredCount = -1

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())
var TerminalWidth = 90
var delayForServiceChanged = 3 * time.Second
var spcIndent = "  "

type TaskDefinition = ecs.TaskDefinition

type TaskDefinitionInput = ecs.RegisterTaskDefinitionInput

type DeploymentDefinitionInput = codedeploy.CreateDeploymentInput

func taskDefinitionName(t *TaskDefinition) string {
	return fmt.Sprintf("%s:%d", *t.Family, *t.Revision)
}

type App struct {
	ecs         *ecs.ECS
	autoScaling *applicationautoscaling.ApplicationAutoScaling
	codedeploy  *codedeploy.CodeDeploy
	cwl         *cloudwatchlogs.CloudWatchLogs
	iam         *iam.IAM

	sess     *session.Session
	verifier *verifier

	Service string
	Cluster string
	config  *Config
	Debug   bool

	ExtStr  map[string]string
	ExtCode map[string]string

	loader *gc.Loader
}

func (d *App) DescribeServicesInput() *ecs.DescribeServicesInput {
	return &ecs.DescribeServicesInput{
		Cluster:  aws.String(d.Cluster),
		Services: []*string{aws.String(d.Service)},
	}
}

func (d *App) DescribeTasksInput(task *ecs.Task) *ecs.DescribeTasksInput {
	return &ecs.DescribeTasksInput{
		Cluster: aws.String(d.Cluster),
		Tasks:   []*string{task.TaskArn},
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

func (d *App) DescribeService(ctx context.Context) (*ecs.Service, error) {
	out, err := d.ecs.DescribeServicesWithContext(ctx, d.DescribeServicesInput())
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe service")
	}
	if len(out.Services) == 0 {
		return nil, errors.New("service is not found")
	}
	return out.Services[0], nil
}

func (d *App) DescribeServiceStatus(ctx context.Context, events int) (*ecs.Service, error) {
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

	if err := d.describeAutoScaling(s); err != nil {
		return nil, errors.Wrap(err, "failed to describe autoscaling")
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

func (d *App) describeAutoScaling(s *ecs.Service) error {
	resourceId := fmt.Sprintf("service/%s/%s", arnToName(*s.ClusterArn), *s.ServiceName)
	tout, err := d.autoScaling.DescribeScalableTargets(
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []*string{&resourceId},
			ServiceNamespace:  aws.String("ecs"),
			ScalableDimension: aws.String("ecs:service:DesiredCount"),
		},
	)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "AccessDeniedException" {
				d.DebugLog("unable to describe scalable targets. requires IAM for application-autoscaling:Describe* to display informations about auto-scaling.")
				return nil
			}
		}
		return errors.Wrap(err, "failed to describe scalable targets")
	}
	if len(tout.ScalableTargets) == 0 {
		return nil
	}

	fmt.Println("AutoScaling:")
	for _, target := range tout.ScalableTargets {
		fmt.Println(formatScalableTarget(target))
	}

	pout, err := d.autoScaling.DescribeScalingPolicies(
		&applicationautoscaling.DescribeScalingPoliciesInput{
			ResourceId:        &resourceId,
			ServiceNamespace:  aws.String("ecs"),
			ScalableDimension: aws.String("ecs:service:DesiredCount"),
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to describe scaling policies")
	}
	for _, policy := range pout.ScalingPolicies {
		fmt.Println(formatScalingPolicy(policy))
	}
	return nil
}

func (d *App) DescribeServiceDeployments(ctx context.Context, startedAt time.Time) (int, error) {
	out, err := d.ecs.DescribeServicesWithContext(ctx, d.DescribeServicesInput())
	if err != nil {
		return 0, err
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

func (d *App) DescribeTaskStatus(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition) error {
	out, err := d.ecs.DescribeTasksWithContext(ctx, d.DescribeTasksInput(task))
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.Log("Task ARN: " + *f.Arn)
		return errors.New(*f.Reason)
	}

	var container *ecs.Container
	for _, c := range out.Tasks[0].Containers {
		if *c.Name == *watchContainer.Name {
			container = c
			break
		}
	}
	if container == nil {
		container = out.Tasks[0].Containers[0]
	}

	if container.ExitCode != nil && *container.ExitCode != 0 {
		msg := fmt.Sprintf("Container: %s, Exit Code: %s", *container.Name, strconv.FormatInt(*container.ExitCode, 10))
		if container.Reason != nil {
			msg += ", Reason: " + *container.Reason
		}
		return errors.New(msg)
	} else if container.Reason != nil {
		return fmt.Errorf("Container: %s, Reason: %s", *container.Name, *container.Reason)
	}
	return nil
}

func (d *App) DescribeTaskDefinition(ctx context.Context, tdArn string) (*TaskDefinitionInput, error) {
	out, err := d.ecs.DescribeTaskDefinitionWithContext(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &tdArn,
		Include:        []*string{aws.String("TAGS")},
	})
	if err != nil {
		return nil, err
	}
	return tdToTaskDefinitionInput(out.TaskDefinition, out.Tags), nil
}

func (d *App) GetLogEvents(ctx context.Context, logGroup string, logStream string, startedAt time.Time, nextToken *string) (*string, error) {
	ms := startedAt.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	out, err := d.cwl.GetLogEventsWithContext(ctx, d.GetLogEventsInput(logGroup, logStream, ms, nextToken))
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

func NewApp(conf *Config) (*App, error) {
	if err := conf.setupPlugins(); err != nil {
		return nil, err
	}
	loader := gc.New()
	for _, f := range conf.templateFuncs {
		loader.Funcs(f)
	}

	sess := conf.sess
	d := &App{
		Service:     conf.Service,
		Cluster:     conf.Cluster,
		ecs:         ecs.New(sess),
		autoScaling: applicationautoscaling.New(sess),
		codedeploy:  codedeploy.New(sess),
		cwl:         cloudwatchlogs.New(sess),
		iam:         iam.New(sess),

		sess:   sess,
		config: conf,
		loader: loader,
	}
	return d, nil
}

func (d *App) Start() (context.Context, context.CancelFunc) {
	log.SetOutput(os.Stdout)

	if d.config.Timeout > 0 {
		return context.WithTimeout(context.Background(), d.config.Timeout)
	} else {
		return context.Background(), func() {}
	}
}

func (d *App) Status(opt StatusOption) error {
	ctx, cancel := d.Start()
	defer cancel()
	_, err := d.DescribeServiceStatus(ctx, *opt.Events)
	return err
}

func (d *App) Delete(opt DeleteOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Deleting service", opt.DryRunString())
	sv, err := d.DescribeServiceStatus(ctx, 3)
	if err != nil {
		return err
	}

	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	if !*opt.Force {
		service := prompter.Prompt(`Enter the service name to DELETE`, "")
		if service != *sv.ServiceName {
			d.Log("Aborted")
			return errors.New("confirmation failed")
		}
	}

	dsi := &ecs.DeleteServiceInput{
		Cluster: sv.ClusterArn,
		Service: sv.ServiceName,
	}
	if _, err := d.ecs.DeleteServiceWithContext(ctx, dsi); err != nil {
		return errors.Wrap(err, "failed to delete service")
	}
	d.Log("Service is deleted")

	return nil
}

func containerOf(td *TaskDefinitionInput, name *string) *ecs.ContainerDefinition {
	if name == nil || *name == "" {
		return td.ContainerDefinitions[0]
	}
	for _, c := range td.ContainerDefinitions {
		if *c.Name == *name {
			c := c
			return c
		}
	}
	return nil
}

func (d *App) Wait(opt WaitOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Waiting for the service stable")

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}
	if isCodeDeploy(sv.DeploymentController) {
		err := d.WaitForCodeDeploy(ctx, sv)
		if err != nil {
			return errors.Wrap(err, "failed to wait for a deployment successfully")
		}
	} else {
		if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
			return errors.Wrap(err, "the service still unstable")
		}
	}
	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) FindRollbackTarget(ctx context.Context, taskDefinitionArn string) (string, error) {
	var found bool
	var nextToken *string
	family := strings.Split(arnToName(taskDefinitionArn), ":")[0]
	for {
		out, err := d.ecs.ListTaskDefinitionsWithContext(ctx,
			&ecs.ListTaskDefinitionsInput{
				NextToken:    nextToken,
				FamilyPrefix: aws.String(family),
				MaxResults:   aws.Int64(100),
				Sort:         aws.String("DESC"),
			},
		)
		if err != nil {
			return "", errors.Wrap(err, "failed to list taskdefinitions")
		}
		if len(out.TaskDefinitionArns) == 0 {
			return "", errors.New("rollback target is not found")
		}
		nextToken = out.NextToken
		for _, tdArn := range out.TaskDefinitionArns {
			if found {
				return *tdArn, nil
			}
			if *tdArn == taskDefinitionArn {
				found = true
			}
		}
	}
}

func (d *App) findLatestTaskDefinitionArn(ctx context.Context, family string) (string, error) {
	out, err := d.ecs.ListTaskDefinitionsWithContext(ctx,
		&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String(family),
			MaxResults:   aws.Int64(1),
			Sort:         aws.String("DESC"),
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to list taskdefinitions")
	}
	if len(out.TaskDefinitionArns) == 0 {
		return "", errors.New("no task definitions are found")
	}
	return *out.TaskDefinitionArns[0], nil
}

func (d *App) Name() string {
	return fmt.Sprintf("%s/%s", d.Service, d.Cluster)
}

func (d *App) Log(v ...interface{}) {
	args := []interface{}{d.Name()}
	args = append(args, v...)
	log.Println(args...)
}

func (d *App) DebugLog(v ...interface{}) {
	if !d.Debug {
		return
	}
	d.Log(v...)
}

func (d *App) LogJSON(v interface{}) {
	fmt.Print(MarshalJSONString(v))
}

func (d *App) WaitServiceStable(ctx context.Context, startedAt time.Time) error {
	d.Log("Waiting for service stable...(it will take a few minutes)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		tick := time.Tick(10 * time.Second)
		var lines int
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick:
				if isTerminal {
					for i := 0; i < lines; i++ {
						fmt.Print(aec.EraseLine(aec.EraseModes.All), aec.PreviousLine(1))
					}
				}
				lines, _ = d.DescribeServiceDeployments(waitCtx, startedAt)
			}
		}
	}()

	return d.ecs.WaitUntilServicesStableWithContext(
		ctx, d.DescribeServicesInput(),
		d.waiterOptions()...,
	)
}

func (d *App) RegisterTaskDefinition(ctx context.Context, td *TaskDefinitionInput) (*TaskDefinition, error) {
	d.Log("Registering a new task definition...")
	if len(td.Tags) == 0 {
		td.Tags = nil // Tags can not be empty.
	}
	out, err := d.ecs.RegisterTaskDefinitionWithContext(
		ctx,
		td,
	)
	if err != nil {
		return nil, err
	}
	d.Log("Task definition is registered", taskDefinitionName(out.TaskDefinition))
	return out.TaskDefinition, nil
}

func (d *App) LoadTaskDefinition(path string) (*TaskDefinitionInput, error) {
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load task definition %s", path)
	}
	c := struct {
		TaskDefinition json.RawMessage `json:"taskDefinition"`
	}{}
	dec := json.NewDecoder(bytes.NewReader(src))
	if err := dec.Decode(&c); err != nil {
		return nil, errors.Wrapf(err, "failed to load task definition %s", path)
	}
	if c.TaskDefinition != nil {
		src = c.TaskDefinition
	}
	var td TaskDefinitionInput
	if err := d.unmarshalJSON(src, &td, path); err != nil {
		return nil, err
	}
	if len(td.Tags) == 0 {
		td.Tags = nil
	}
	return &td, nil
}

func (d *App) LoadDeploymentDefinition(path string) (*DeploymentDefinitionInput, error) {
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load deployment definition %s", path)
	}
	c := struct {
		DeploymentDefinition json.RawMessage `json:"deploymentDefinition"`
	}{}
	dec := json.NewDecoder(bytes.NewReader(src))
	if err := dec.Decode(&c); err != nil {
		return nil, errors.Wrapf(err, "failed to load deployment definition %s", path)
	}
	if c.DeploymentDefinition != nil {
		src = c.DeploymentDefinition
	}
	var dd DeploymentDefinitionInput
	if err := d.unmarshalJSON(src, &dd, path); err != nil {
		return nil, err
	}
	return &dd, nil
}

func (d *App) unmarshalJSON(src []byte, v interface{}, path string) error {
	strict := json.NewDecoder(bytes.NewReader(src))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&v); err != nil {
		if !strings.Contains(err.Error(), "unknown field") {
			return err
		}
		fmt.Fprintln(
			os.Stderr,
			color.YellowString("WARNING: %s in %s", err, path),
		)
		// unknown field -> try lax decoder
		lax := json.NewDecoder(bytes.NewReader(src))
		return lax.Decode(&v)
	}
	return nil
}

func (d *App) LoadServiceDefinition(path string) (*ecs.Service, error) {
	if path == "" {
		return nil, errors.New("service_definition is not defined")
	}

	sv := ecs.Service{}
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load service definition %s", path)
	}
	if err := d.unmarshalJSON(src, &sv, path); err != nil {
		return nil, errors.Wrapf(err, "failed to load service definition %s", path)
	}

	sv.ServiceName = aws.String(d.config.Service)
	return &sv, nil
}

func (d *App) GetLogInfo(task *ecs.Task, c *ecs.ContainerDefinition) (string, string) {
	p := strings.Split(*task.TaskArn, "/")
	taskID := p[len(p)-1]
	lc := c.LogConfiguration
	logStreamPrefix := *lc.Options["awslogs-stream-prefix"]

	logStream := strings.Join([]string{logStreamPrefix, *c.Name, taskID}, "/")
	logGroup := *lc.Options["awslogs-group"]

	d.Log("logGroup:", logGroup)
	d.Log("logStream:", logStream)

	return logGroup, logStream
}

func (d *App) suspendAutoScaling(suspendState bool) error {
	resourceId := fmt.Sprintf("service/%s/%s", d.Cluster, d.Service)

	out, err := d.autoScaling.DescribeScalableTargets(
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []*string{&resourceId},
			ServiceNamespace:  aws.String("ecs"),
			ScalableDimension: aws.String("ecs:service:DesiredCount"),
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to describe scalable targets")
	}
	if len(out.ScalableTargets) == 0 {
		d.Log(fmt.Sprintf("No scalable target for %s", resourceId))
		return nil
	}
	for _, target := range out.ScalableTargets {
		d.Log(fmt.Sprintf("Register scalable target %s set suspend state to %t", *target.ResourceId, suspendState))
		_, err := d.autoScaling.RegisterScalableTarget(
			&applicationautoscaling.RegisterScalableTargetInput{
				ServiceNamespace:  target.ServiceNamespace,
				ScalableDimension: target.ScalableDimension,
				ResourceId:        target.ResourceId,
				SuspendedState: &applicationautoscaling.SuspendedState{
					DynamicScalingInSuspended:  &suspendState,
					DynamicScalingOutSuspended: &suspendState,
					ScheduledScalingSuspended:  &suspendState,
				},
			},
		)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to register scalable target %s set suspend state to %t", *target.ResourceId, suspendState))
		}
	}
	return nil
}

func (d *App) WaitForCodeDeploy(ctx context.Context, sv *ecs.Service) error {
	dp, err := d.findDeploymentInfo()
	if err != nil {
		return err
	}
	out, err := d.codedeploy.ListDeploymentsWithContext(
		ctx,
		&codedeploy.ListDeploymentsInput{
			ApplicationName:     dp.ApplicationName,
			DeploymentGroupName: dp.DeploymentGroupName,
			IncludeOnlyStatuses: []*string{
				aws.String("Created"),
				aws.String("Queued"),
				aws.String("InProgress"),
				aws.String("Ready"),
			},
		},
	)
	if err != nil {
		return err
	}
	if len(out.Deployments) == 0 {
		return errors.New("no deployments found in progress")
	}
	dpID := out.Deployments[0]
	d.Log("Waiting for a deployment successful ID: " + *dpID)
	return d.codedeploy.WaitUntilDeploymentSuccessfulWithContext(
		ctx,
		&codedeploy.GetDeploymentInput{DeploymentId: dpID},
		d.waiterOptions()...,
	)
}

func (d *App) RollbackByCodeDeploy(ctx context.Context, sv *ecs.Service, tdArn string, opt RollbackOption) error {
	dp, err := d.findDeploymentInfo()
	if err != nil {
		return err
	}

	ld, err := d.codedeploy.ListDeploymentsWithContext(ctx, &codedeploy.ListDeploymentsInput{
		ApplicationName:     dp.ApplicationName,
		DeploymentGroupName: dp.DeploymentGroupName,
	})
	if err != nil {
		return errors.Wrap(err, "failed to list deployments")
	}
	if len(ld.Deployments) == 0 {
		return errors.New("no deployments are found")
	}

	dpID := ld.Deployments[0] // latest deployment id

	dep, err := d.codedeploy.GetDeploymentWithContext(ctx, &codedeploy.GetDeploymentInput{
		DeploymentId: dpID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to get deployment")
	}

	if *opt.DryRun {
		d.Log("deployment id:", *dpID)
		d.Log("DRY RUN OK")
		return nil
	}

	switch *dep.DeploymentInfo.Status {
	case "Succeeded", "Failed", "Stopped":
		// Add default value
		ddi := &DeploymentDefinitionInput{}
		return d.createDeployment(ctx, sv, tdArn, ddi, opt.RollbackEvents)
	default: // If the deployment is not yet complete
		_, err = d.codedeploy.StopDeploymentWithContext(ctx, &codedeploy.StopDeploymentInput{
			DeploymentId:        dpID,
			AutoRollbackEnabled: aws.Bool(true),
		})
		if err != nil {
			return errors.Wrap(err, "failed to roll back the deployment")
		}

		d.Log(fmt.Sprintf("Deployment %s is rolled back on CodeDeploy:", *dpID))
		return nil
	}
}

// Build an option WithWaiterDelay and request.WithWaiterMaxAttempts for a long timeout.
// SDK Default is 10 min (MaxAttempts=40 * Delay=15sec) at now.
// ref. https://github.com/aws/aws-sdk-go/blob/d57c8d96f72d9475194ccf18d2ba70ac294b0cb3/service/ecs/waiters.go#L82-L83
// Explicitly set these options so not being affected by the default setting.
func (d *App) waiterOptions() []request.WaiterOption {
	const delay = 15 * time.Second
	attempts := int((d.config.Timeout / delay)) + 1
	if (d.config.Timeout % delay) > 0 {
		attempts++
	}
	return []request.WaiterOption{
		request.WithWaiterDelay(request.ConstantWaiterDelay(delay)),
		request.WithWaiterMaxAttempts(attempts),
	}
}
