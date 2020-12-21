package ecspresso

import (
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
	goconfig "github.com/kayac/go-config"
	"github.com/mattn/go-isatty"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
)

const DefaultDesiredCount = -1

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())
var TerminalWidth = 90
var delayForServiceChanged = 3 * time.Second
var spcIndent = "  "

func taskDefinitionName(t *ecs.TaskDefinition) string {
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

	loader *goconfig.Loader
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

func (d *App) GetLogEventsInput(logGroup string, logStream string, startAt int64) *cloudwatchlogs.GetLogEventsInput {
	return &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(startAt),
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
	resouceId := fmt.Sprintf("service/%s/%s", arnToName(*s.ClusterArn), *s.ServiceName)
	tout, err := d.autoScaling.DescribeScalableTargets(
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []*string{&resouceId},
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
			ResourceId:        &resouceId,
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

func (d *App) DescribeTaskDefinition(ctx context.Context, tdArn string) (*ecs.TaskDefinition, error) {
	out, err := d.ecs.DescribeTaskDefinitionWithContext(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &tdArn,
	})
	if err != nil {
		return nil, err
	}
	return out.TaskDefinition, nil
}

func (d *App) GetLogEvents(ctx context.Context, logGroup string, logStream string, startedAt time.Time) (int, error) {
	ms := startedAt.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	out, err := d.cwl.GetLogEventsWithContext(ctx, d.GetLogEventsInput(logGroup, logStream, ms))
	if err != nil {
		return 0, err
	}
	if len(out.Events) == 0 {
		return 0, nil
	}
	lines := 0
	for _, event := range out.Events {
		for _, line := range formatLogEvent(event, TerminalWidth) {
			fmt.Println(line)
			lines++
		}
	}
	return lines, nil
}

func NewApp(conf *Config) (*App, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String(conf.Region)},
		SharedConfigState: session.SharedConfigEnable,
	}))
	if err := conf.Setup(sess); err != nil {
		return nil, err
	}

	loader := goconfig.New()
	for _, f := range conf.templateFuncs {
		loader.Funcs(f)
	}

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

func (d *App) Create(opt CreateOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting create service", opt.DryRunString())
	svd, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load service definition")
	}
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}

	count := calcDesiredCount(svd, opt)
	if count == nil && (svd.SchedulingStrategy != nil && *svd.SchedulingStrategy == "REPLICA") {
		count = aws.Int64(1) // Must provide desired count for replica scheduling strategy
	}

	if *opt.DryRun {
		d.Log("task definition:")
		d.LogJSON(td)
		d.Log("service definition:")
		d.LogJSON(svd)
		d.Log("DRY RUN OK")
		return nil
	}

	newTd, err := d.RegisterTaskDefinition(ctx, td)
	if err != nil {
		return errors.Wrap(err, "failed to register task definition")
	}
	createServiceInput := &ecs.CreateServiceInput{
		Cluster:                       aws.String(d.config.Cluster),
		CapacityProviderStrategy:      svd.CapacityProviderStrategy,
		DeploymentConfiguration:       svd.DeploymentConfiguration,
		DeploymentController:          svd.DeploymentController,
		DesiredCount:                  count,
		EnableECSManagedTags:          svd.EnableECSManagedTags,
		HealthCheckGracePeriodSeconds: svd.HealthCheckGracePeriodSeconds,
		LaunchType:                    svd.LaunchType,
		LoadBalancers:                 svd.LoadBalancers,
		NetworkConfiguration:          svd.NetworkConfiguration,
		PlacementConstraints:          svd.PlacementConstraints,
		PlacementStrategy:             svd.PlacementStrategy,
		PlatformVersion:               svd.PlatformVersion,
		PropagateTags:                 svd.PropagateTags,
		SchedulingStrategy:            svd.SchedulingStrategy,
		ServiceName:                   svd.ServiceName,
		ServiceRegistries:             svd.ServiceRegistries,
		Tags:                          svd.Tags,
		TaskDefinition:                newTd.TaskDefinitionArn,
	}
	if _, err := d.ecs.CreateServiceWithContext(ctx, createServiceInput); err != nil {
		return errors.Wrap(err, "failed to create service")
	}
	d.Log("Service is created")

	if *opt.NoWait {
		return nil
	}

	start := time.Now()
	time.Sleep(delayForServiceChanged) // wait for service created
	if err := d.WaitServiceStable(ctx, start); err != nil {
		return errors.Wrap(err, "failed to wait service stable")
	}

	d.Log("Service is stable now. Completed!")
	return nil
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

func containerOf(td *ecs.TaskDefinition, name *string) *ecs.ContainerDefinition {
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

func (d *App) Run(opt RunOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Running task", opt.DryRunString())
	var ov ecs.TaskOverride
	if ovStr := *opt.TaskOverrideStr; ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return errors.Wrap(err, "invalid overrides")
		}
	}

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "failed to describe service status")
	}

	var tdArn string
	var watchContainer *ecs.ContainerDefinition

	if *opt.LatestTaskDefinition {
		family := strings.Split(arnToName(*sv.TaskDefinition), ":")[0]
		var err error
		tdArn, err = d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return errors.Wrap(err, "failed to load latest task definition")
		}

		td, err := d.DescribeTaskDefinition(ctx, tdArn)
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		watchContainer = containerOf(td, opt.WatchContainer)
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		}
	} else if *opt.SkipTaskDefinition {
		td, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		tdArn = *(td.TaskDefinitionArn)
		watchContainer = containerOf(td, opt.WatchContainer)
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		}
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load task definition")
		}

		if len(*opt.TaskDefinition) > 0 {
			d.Log("Loading task definition")
			runTd, err := d.LoadTaskDefinition(*opt.TaskDefinition)
			if err != nil {
				return errors.Wrap(err, "failed to load task definition")
			}
			td = runTd
		}

		var newTd *ecs.TaskDefinition
		_ = newTd

		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		} else {
			newTd, err = d.RegisterTaskDefinition(ctx, td)
			if err != nil {
				return errors.Wrap(err, "failed to register task definition")
			}
			tdArn = *newTd.TaskDefinitionArn
			watchContainer = containerOf(td, opt.WatchContainer)
		}
	}
	if watchContainer == nil {
		return fmt.Errorf("container %s is not found in task definition", *opt.WatchContainer)
	}
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	task, err := d.RunTask(ctx, tdArn, sv, &ov, *opt.Count)
	if err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	if *opt.NoWait {
		d.Log("Run task invoked")
		return nil
	}
	d.Log(fmt.Sprintf("Watching container: %s", *watchContainer.Name))
	if err := d.WaitRunTask(ctx, task, watchContainer, time.Now()); err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	if err := d.DescribeTaskStatus(ctx, task, watchContainer); err != nil {
		return err
	}
	d.Log("Run task completed!")

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

	// Add an option WithWaiterDelay and request.WithWaiterMaxAttempts for a long timeout.
	// SDK Default is 10 min (MaxAttempts=40 * Delay=15sec) at now.
	// ref. https://github.com/aws/aws-sdk-go/blob/d57c8d96f72d9475194ccf18d2ba70ac294b0cb3/service/ecs/waiters.go#L82-L83
	// Explicitly set these options so not being affected by the default setting.
	const delay = 15 * time.Second
	attempts := int((d.config.Timeout / delay)) + 1
	if (d.config.Timeout % delay) > 0 {
		attempts++
	}
	return d.ecs.WaitUntilServicesStableWithContext(
		ctx, d.DescribeServicesInput(),
		request.WithWaiterDelay(request.ConstantWaiterDelay(delay)),
		request.WithWaiterMaxAttempts(attempts),
	)
}

func (d *App) RegisterTaskDefinition(ctx context.Context, td *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	d.Log("Registering a new task definition...")

	out, err := d.ecs.RegisterTaskDefinitionWithContext(
		ctx,
		tdToRegisterTaskDefinitionInput(td),
	)
	if err != nil {
		return nil, err
	}
	d.Log("Task definition is registered", taskDefinitionName(out.TaskDefinition))
	return out.TaskDefinition, nil
}

func (d *App) LoadTaskDefinition(path string) (*ecs.TaskDefinition, error) {
	c := struct {
		TaskDefinition *ecs.TaskDefinition
	}{}
	if err := d.loader.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}
	if c.TaskDefinition != nil {
		return c.TaskDefinition, nil
	}
	var td ecs.TaskDefinition
	if err := d.loader.LoadWithEnvJSON(&td, path); err != nil {
		return nil, err
	}
	return &td, nil
}

func (d *App) LoadServiceDefinition(path string) (*ecs.Service, error) {
	if path == "" {
		return nil, errors.New("service_definition is not defined")
	}

	c := ecs.Service{}
	if err := d.loader.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}

	c.ServiceName = aws.String(d.config.Service)

	return &c, nil
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

func (d *App) RunTask(ctx context.Context, tdArn string, sv *ecs.Service, ov *ecs.TaskOverride, count int64) (*ecs.Task, error) {
	d.Log("Running task")

	out, err := d.ecs.RunTaskWithContext(
		ctx,
		&ecs.RunTaskInput{
			Cluster:                  aws.String(d.Cluster),
			TaskDefinition:           aws.String(tdArn),
			NetworkConfiguration:     sv.NetworkConfiguration,
			LaunchType:               sv.LaunchType,
			Overrides:                ov,
			Count:                    aws.Int64(count),
			CapacityProviderStrategy: sv.CapacityProviderStrategy,
			PlacementConstraints:     sv.PlacementConstraints,
			PlacementStrategy:        sv.PlacementStrategy,
			PlatformVersion:          sv.PlatformVersion,
		},
	)
	if err != nil {
		return nil, err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.Log("Task ARN: " + *f.Arn)
		return nil, errors.New(*f.Reason)
	}

	task := out.Tasks[0]
	d.Log("Task ARN:", *task.TaskArn)
	return task, nil
}

func (d *App) WaitRunTask(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition, startedAt time.Time) error {
	d.Log("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := watchContainer.LogConfiguration
	if lc == nil || *lc.LogDriver != "awslogs" || lc.Options["awslogs-stream-prefix"] == nil {
		d.Log("awslogs not configured")
		if err := d.WaitUntilTaskStopped(ctx, task); err != nil {
			return errors.Wrap(err, "failed to run task")
		}
		return nil
	}

	logGroup, logStream := d.GetLogInfo(task, watchContainer)
	time.Sleep(3 * time.Second) // wait for log stream

	go func() {
		tick := time.Tick(5 * time.Second)
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
				lines, _ = d.GetLogEvents(waitCtx, logGroup, logStream, startedAt)
			}
		}
	}()

	if err := d.WaitUntilTaskStopped(ctx, task); err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	return nil
}

func (d *App) WaitUntilTaskStopped(ctx context.Context, task *ecs.Task) error {
	// Add an option WithWaiterDelay and request.WithWaiterMaxAttempts for a long timeout.
	// SDK Default is 10 min (MaxAttempts=100 * Delay=6sec) at now.
	const delay = 6 * time.Second
	attempts := int((d.config.Timeout / delay)) + 1
	if (d.config.Timeout % delay) > 0 {
		attempts++
	}
	return d.ecs.WaitUntilTasksStoppedWithContext(
		ctx, d.DescribeTasksInput(task),
		request.WithWaiterDelay(request.ConstantWaiterDelay(delay)),
		request.WithWaiterMaxAttempts(attempts),
	)
}

func (d *App) Register(opt RegisterOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting register task definition", opt.DryRunString())
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}
	if *opt.DryRun {
		d.Log("task definition:")
		d.LogJSON(td)
		d.Log("DRY RUN OK")
		return nil
	}

	newTd, err := d.RegisterTaskDefinition(ctx, td)
	if err != nil {
		return errors.Wrap(err, "failed to register task definition")
	}

	if *opt.Output {
		d.LogJSON(newTd)
	}
	return nil
}

func (d *App) suspendAutoScaling(suspend bool) error {
	resouceId := fmt.Sprintf("service/%s/%s", d.Cluster, d.Service)

	out, err := d.autoScaling.DescribeScalableTargets(
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []*string{&resouceId},
			ServiceNamespace:  aws.String("ecs"),
			ScalableDimension: aws.String("ecs:service:DesiredCount"),
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to describe scalable targets")
	}
	if len(out.ScalableTargets) == 0 {
		d.Log(fmt.Sprintf("No scalable target for %s", resouceId))
		return nil
	}
	for _, target := range out.ScalableTargets {
		d.Log(fmt.Sprintf("Register scalable target %s set suspend to %t", *target.ResourceId, suspend))
		_, err := d.autoScaling.RegisterScalableTarget(
			&applicationautoscaling.RegisterScalableTargetInput{
				ServiceNamespace:  target.ServiceNamespace,
				ScalableDimension: target.ScalableDimension,
				ResourceId:        target.ResourceId,
				SuspendedState: &applicationautoscaling.SuspendedState{
					DynamicScalingInSuspended:  &suspend,
					DynamicScalingOutSuspended: &suspend,
					ScheduledScalingSuspended:  &suspend,
				},
			},
		)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to register scalable target %s set suspend to %t", *target.ResourceId, suspend))
		}
	}
	return nil
}

func (d *App) WaitForCodeDeploy(ctx context.Context, sv *ecs.Service) error {
	dp, err := d.findDeploymentInfo(sv)
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
	)
}
