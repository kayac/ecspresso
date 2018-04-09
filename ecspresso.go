package ecspresso

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/kayac/go-config"
	"github.com/mattn/go-isatty"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
)

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())
var TerminalWidth = 90

const KeepDesiredCount = -1

func taskDefinitionName(t *ecs.TaskDefinition) string {
	return fmt.Sprintf("%s:%d", *t.Family, *t.Revision)
}

type App struct {
	ecs     *ecs.ECS
	cwl     *cloudwatchlogs.CloudWatchLogs
	cwe     *cloudwatchevents.CloudWatchEvents
	Service string
	Cluster string
	config  *Config
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

func (d *App) DescribeServiceStatus(ctx context.Context, events int) (*ecs.Service, error) {
	out, err := d.ecs.DescribeServicesWithContext(ctx, d.DescribeServicesInput())
	if err != nil {
		return nil, errors.Wrap(err, "describe services failed")
	}
	if len(out.Services) == 0 {
		return nil, errors.New("no services found")
	}
	s := out.Services[0]
	fmt.Println("Service:", *s.ServiceName)
	fmt.Println("Cluster:", arnToName(*s.ClusterArn))
	fmt.Println("TaskDefinition:", arnToName(*s.TaskDefinition))
	fmt.Println("Deployments:")
	for _, dep := range s.Deployments {
		fmt.Println("  ", formatDeployment(dep))
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

func (d *App) DescribeTask(ctx context.Context, task *ecs.Task) error {
	d.Log("Checking if task ran successfully")
	out, err := d.ecs.DescribeTasksWithContext(ctx, d.DescribeTasksInput(task))
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.Log("Task ARN: " + *f.Arn)
		return errors.New(*f.Reason)
	}

	c := out.Tasks[0].Containers[0]
	if c.Reason != nil {
		return errors.New(*c.Reason)
	}
	if c.ExitCode != nil && *c.ExitCode != 0 {
		msg := "Exit Code: " + strconv.FormatInt(*c.ExitCode, 10)
		if c.Reason != nil {
			msg += ", Reason: " + *c.Reason
		}
		return errors.New(msg)
	}
	return nil
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
	if err := conf.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}
	sess := session.Must(session.NewSession(
		&aws.Config{Region: aws.String(conf.Region)},
	))
	d := &App{
		Service: conf.Service,
		Cluster: conf.Cluster,
		ecs:     ecs.New(sess),
		cwl:     cloudwatchlogs.New(sess),
		cwe:     cloudwatchevents.New(sess),
		config:  conf,
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

	d.Log("Starting create service")
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "create failed")
	}
	svd, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "create failed")
	}

	if *opt.DesiredCount != 1 {
		svd.DesiredCount = opt.DesiredCount
	}

	if *opt.DryRun {
		d.Log("task definition:", td.String())
		d.Log("service definition:", svd.String())
		d.Log("DRY RUN OK")
		return nil
	}

	newTd, err := d.RegisterTaskDefinition(ctx, td)
	if err != nil {
		return errors.Wrap(err, "create failed")
	}
	svd.TaskDefinition = newTd.TaskDefinitionArn

	if _, err := d.ecs.CreateServiceWithContext(ctx, svd); err != nil {
		return errors.Wrap(err, "create failed")
	}
	d.Log("Service is created")

	start := time.Now()
	time.Sleep(3 * time.Second) // wait for service created
	if err := d.WaitServiceStable(ctx, start); err != nil {
		return errors.Wrap(err, "create failed")
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) Delete(opt DeleteOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Deleting service")
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
		return errors.Wrap(err, "delete failed")
	}
	d.Log("Service is deleted")

	return nil
}

func (d *App) Run(opt RunOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "run failed")
	}

	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "run failed")
	}

	if len(*opt.TaskDefinition) > 0 {
		d.Log("Loading task definition")
		runTd, err := d.LoadTaskDefinition(*opt.TaskDefinition)
		if err != nil {
			return errors.Wrap(err, "run failed")
		}
		td = runTd
	}

	var newTd *ecs.TaskDefinition
	_ = newTd

	if *opt.DryRun {
		d.Log("task definition:", td.String())
	} else {
		newTd, err = d.RegisterTaskDefinition(ctx, td)
		if err != nil {
			return errors.Wrap(err, "run failed")
		}
	}

	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	tdArn := *newTd.TaskDefinitionArn
	lc := newTd.ContainerDefinitions[0].LogConfiguration

	task, err := d.RunTask(ctx, tdArn, sv)
	if err != nil {
		return errors.Wrap(err, "run failed")
	}
	if err := d.WaitRunTask(ctx, task, lc, time.Now()); err != nil {
		return errors.Wrap(err, "run failed")
	}
	if err := d.DescribeTask(ctx, task); err != nil {
		return errors.Wrap(err, "run failed")
	}
	d.Log("Run task completed!")

	return nil
}

func (d *App) Deploy(opt DeployOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting deploy")
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "deploy failed")
	}

	var count *int64
	if *opt.DesiredCount == KeepDesiredCount {
		count = sv.DesiredCount
	} else {
		count = opt.DesiredCount
	}

	var tdArn string
	if *opt.SkipTaskDefinition {
		tdArn = *sv.TaskDefinition
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "deploy failed")
		}
		if *opt.DryRun {
			d.Log("task definition:", td.String())
		} else {
			newTd, err := d.RegisterTaskDefinition(ctx, td)
			if err != nil {
				return errors.Wrap(err, "deploy failed")
			}
			tdArn = *newTd.TaskDefinitionArn
		}
	}
	d.Log("desired count:", *count)
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	if err := d.UpdateService(ctx, tdArn, *count, *opt.ForceNewDeployment, sv); err != nil {
		return errors.Wrap(err, "deploy failed")
	}
	if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
		return errors.Wrap(err, "deploy failed")
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) Rollback(opt RollbackOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting rollback")
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "rollback failed")
	}
	currentArn := *sv.TaskDefinition
	targetArn, err := d.FindRollbackTarget(ctx, currentArn)
	if err != nil {
		return errors.Wrap(err, "rollback failed")
	}
	d.Log("Rollbacking to", arnToName(targetArn))
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	if err := d.UpdateService(ctx, targetArn, *sv.DesiredCount, false, sv); err != nil {
		return errors.Wrap(err, "rollback failed")
	}
	if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
		return errors.Wrap(err, "rollback failed")
	}

	d.Log("Service is stable now. Completed!")

	if *opt.DeregisterTaskDefinition {
		d.Log("Deregistering rolled back task definition", arnToName(currentArn))
		_, err := d.ecs.DeregisterTaskDefinitionWithContext(
			ctx,
			&ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: &currentArn,
			},
		)
		if err != nil {
			return errors.Wrap(err, "deregister task definition failed")
		}
		d.Log(arnToName(currentArn), "was deregistered successfully")
	}

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
			return "", errors.Wrap(err, "list taskdefinitions failed")
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

func (d *App) Name() string {
	return fmt.Sprintf("%s/%s", d.Service, d.Cluster)
}

func (d *App) Log(v ...interface{}) {
	args := []interface{}{d.Name()}
	args = append(args, v...)
	log.Println(args...)
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

	return d.ecs.WaitUntilServicesStableWithContext(ctx, d.DescribeServicesInput())
}

func (d *App) UpdateService(ctx context.Context, taskDefinitionArn string, count int64, force bool, sv *ecs.Service) error {
	msg := "Updating service"
	if force {
		msg = msg + " with force new deployment"
	}
	msg = msg + "..."
	d.Log(msg)

	_, err := d.ecs.UpdateServiceWithContext(
		ctx,
		&ecs.UpdateServiceInput{
			Service:              aws.String(d.Service),
			Cluster:              aws.String(d.Cluster),
			TaskDefinition:       aws.String(taskDefinitionArn),
			DesiredCount:         &count,
			ForceNewDeployment:   &force,
			NetworkConfiguration: sv.NetworkConfiguration,
		},
	)
	return err
}

func (d *App) RegisterTaskDefinition(ctx context.Context, td *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	d.Log("Registering a new task definition...")

	out, err := d.ecs.RegisterTaskDefinitionWithContext(
		ctx,
		&ecs.RegisterTaskDefinitionInput{
			ContainerDefinitions:    td.ContainerDefinitions,
			Cpu:                     td.Cpu,
			ExecutionRoleArn:        td.ExecutionRoleArn,
			Family:                  td.Family,
			Memory:                  td.Memory,
			NetworkMode:             td.NetworkMode,
			PlacementConstraints:    td.PlacementConstraints,
			RequiresCompatibilities: td.RequiresCompatibilities,
			TaskRoleArn:             td.TaskRoleArn,
			Volumes:                 td.Volumes,
		},
	)
	if err != nil {
		return nil, err
	}
	d.Log("Task definition is registered", taskDefinitionName(out.TaskDefinition))
	return out.TaskDefinition, nil
}

func (d *App) LoadTaskDefinition(path string) (*ecs.TaskDefinition, error) {
	d.Log("Creating a new task definition by", path)
	c := struct {
		TaskDefinition *ecs.TaskDefinition
	}{}
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}
	if c.TaskDefinition != nil {
		return c.TaskDefinition, nil
	}
	var td ecs.TaskDefinition
	if err := config.LoadWithEnvJSON(&td, path); err != nil {
		return nil, err
	}
	return &td, nil
}

func (d *App) LoadServiceDefinition(path string) (*ecs.CreateServiceInput, error) {
	c := ServiceDefinition{}
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}

	var count *int64
	if c.DesiredCount == nil {
		count = aws.Int64(1)
	} else {
		count = c.DesiredCount
	}

	return &ecs.CreateServiceInput{
		Cluster:                 aws.String(d.config.Cluster),
		DesiredCount:            count,
		ServiceName:             aws.String(d.config.Service),
		DeploymentConfiguration: c.DeploymentConfiguration,
		LaunchType:              c.LaunchType,
		LoadBalancers:           c.LoadBalancers,
		NetworkConfiguration:    c.NetworkConfiguration,
		PlacementConstraints:    c.PlacementConstraints,
		PlacementStrategy:       c.PlacementStrategy,
		Role:                    c.Role,
	}, nil
}

func (d *App) GetLogInfo(task *ecs.Task, lc *ecs.LogConfiguration) (string, string) {
	taskId := strings.Split(*task.TaskArn, "/")[1]
	logStreamPrefix := *lc.Options["awslogs-stream-prefix"]
	containerName := *task.Containers[0].Name

	logStream := strings.Join([]string{logStreamPrefix, containerName, taskId}, "/")
	logGroup := *lc.Options["awslogs-group"]

	d.Log("logGroup:", logGroup)
	d.Log("logStream:", logStream)

	return logGroup, logStream
}

func (d *App) RunTask(ctx context.Context, tdArn string, sv *ecs.Service) (*ecs.Task, error) {
	d.Log("Running task")

	out, err := d.ecs.RunTaskWithContext(
		ctx,
		&ecs.RunTaskInput{
			Cluster:              aws.String(d.Cluster),
			TaskDefinition:       aws.String(tdArn),
			NetworkConfiguration: sv.NetworkConfiguration,
			LaunchType:           sv.LaunchType,
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

func (d *App) WaitRunTask(ctx context.Context, task *ecs.Task, lc *ecs.LogConfiguration, startedAt time.Time) error {
	d.Log("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if lc == nil || *lc.LogDriver != "awslogs" || lc.Options["awslogs-stream-prefix"] == nil {
		d.Log("awslogs not configured")
		return d.ecs.WaitUntilTasksStoppedWithContext(ctx, d.DescribeTasksInput(task))
	}

	logGroup, logStream := d.GetLogInfo(task, lc)
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
	return d.ecs.WaitUntilTasksStoppedWithContext(ctx, d.DescribeTasksInput(task))
}

func (d *App) LoadRuleDefinition(path string) (*cloudwatchevents.PutRuleInput, error) {
	c := cloudwatchevents.PutRuleInput{}
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *App) LoadTargetDefinition(path string) (*cloudwatchevents.Target, error) {
	c := cloudwatchevents.Target{}
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *App) DescribeTaskDefinition(ctx context.Context, td *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	out, err := d.ecs.DescribeTaskDefinitionWithContext(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: td.Family})
	if err != nil {
		return nil, errors.New("no task-definition found")
	}
	return out.TaskDefinition, nil
}

func (d *App) DescribeCluster(ctx context.Context, name string) (*ecs.Cluster, error) {
	out, err := d.ecs.DescribeClustersWithContext(ctx, &ecs.DescribeClustersInput{Clusters: []*string{aws.String(name)}})
	if err != nil {
		return nil, errors.New("no cluster found")
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.Log("Cluster ARN: " + *f.Arn)
		return nil, errors.New(*f.Reason)
	}
	return out.Clusters[0], nil
}

func (d *App) SchedulerPut(opt SchedulerPutOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting put task scheduler")

	c, err := d.DescribeCluster(ctx, d.config.Cluster)
	if err != nil {
		return errors.Wrap(err, "put task scheduler failed")
	}

	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "put task scheduler failed")
	}

	if *opt.SkipTaskDefinition || *opt.DryRun {
		td, err = d.DescribeTaskDefinition(ctx, td)
		if err != nil {
			return errors.Wrap(err, "put task scheduler failed")
		}
	} else {
		newTd, err := d.RegisterTaskDefinition(ctx, td)
		if err != nil {
			return errors.Wrap(err, "put task scheduler failed")
		}
		td = newTd
	}

	rd, err := d.LoadRuleDefinition(d.config.RuleDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "put task scheduler failed")
	}

	ttd, err := d.LoadTargetDefinition(d.config.TargetDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "put task scheduler failed")
	}

	ttd.EcsParameters.TaskDefinitionArn = td.TaskDefinitionArn
	ttd.Arn = c.ClusterArn
	tsd := &cloudwatchevents.PutTargetsInput{
		Rule:    rd.Name,
		Targets: []*cloudwatchevents.Target{ttd},
	}

	if *opt.DryRun {
		d.Log("task definition:", td.String())
		d.Log("rule definition", rd.String())
		d.Log("target definition", tsd.String())
		d.Log("DRY RUN OK")
		return nil
	}

	if _, err := d.cwe.PutRuleWithContext(ctx, rd); err != nil {
		return errors.Wrap(err, "put task scheduler failed")
	}

	if _, err := d.cwe.PutTargetsWithContext(ctx, tsd); err != nil {
		return errors.Wrap(err, "put task scheduler failed")
	}

	d.Log("task scheduler is put")

	return nil
}

func (d *App) SchedulerDelete(opt SchedulerDeleteOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting delete task scheduler")

	rd, err := d.LoadRuleDefinition(d.config.RuleDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "delete task scheduler failed")
	}

	ttd, err := d.LoadTargetDefinition(d.config.TargetDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "delete task scheduler failed")
	}

	if *opt.DryRun {
		d.Log("rule definition", rd.String())
		d.Log("target definition", ttd.String())
		d.Log("DRY RUN OK")
		return nil
	}

	if !*opt.Force {
		service := prompter.Prompt(`Enter the rule name to DELETE`, "")
		if service != *rd.Name {
			d.Log("Aborted")
			return errors.New("confirmation failed")
		}
	}

	_, err = d.cwe.RemoveTargetsWithContext(ctx, &cloudwatchevents.RemoveTargetsInput{Rule: rd.Name, Ids: []*string{ttd.Id}})
	if err != nil {
		return errors.Wrap(err, "delete task scheduler failed")
	}

	_, err = d.cwe.DeleteRuleWithContext(ctx, &cloudwatchevents.DeleteRuleInput{Name: rd.Name})
	if err != nil {
		return errors.Wrap(err, "delete task scheduler failed")
	}

	d.Log("task scheduler is deleted")

	return nil
}

func (d *App) SchedulerEnable(opt SchedulerEnableOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting enable task scheduler")

	rd, err := d.LoadRuleDefinition(d.config.RuleDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "enable task scheduler failed")
	}

	if *opt.DryRun {
		d.Log("rule definition", rd.String())
		d.Log("DRY RUN OK")
		return nil
	}

	_, err = d.cwe.EnableRuleWithContext(ctx, &cloudwatchevents.EnableRuleInput{Name: rd.Name})
	if err != nil {
		return errors.Wrap(err, "enable task scheduler failed")
	}

	d.Log("task scheduler is enabled")

	return nil
}

func (d *App) SchedulerDisable(opt SchedulerDisableOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting disable task scheduler")

	rd, err := d.LoadRuleDefinition(d.config.RuleDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "disable task scheduler failed")
	}

	if *opt.DryRun {
		d.Log("rule definition", rd.String())
		d.Log("DRY RUN OK")
		return nil
	}

	_, err = d.cwe.DisableRuleWithContext(ctx, &cloudwatchevents.DisableRuleInput{Name: rd.Name})
	if err != nil {
		return errors.Wrap(err, "disable task scheduler failed")
	}

	d.Log("task scheduler is disabled")

	return nil
}
