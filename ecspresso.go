package ecspresso

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/kayac/go-config"
	"github.com/mattn/go-isatty"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
)

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())

func taskDefinitionName(t *ecs.TaskDefinition) string {
	return fmt.Sprintf("%s:%d", *t.Family, *t.Revision)
}

type App struct {
	ecs            *ecs.ECS
	Service        string
	Cluster        string
	TaskDefinition *ecs.TaskDefinition
	Registered     *ecs.TaskDefinition
	config         *Config
}

func (d *App) DescribeServicesInput() *ecs.DescribeServicesInput {
	return &ecs.DescribeServicesInput{
		Cluster:  aws.String(d.Cluster),
		Services: []*string{aws.String(d.Service)},
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
		fmt.Println("  ", formatEvent(event))
	}
	return s, nil
}

func (d *App) DescribeServiceDeployments(ctx context.Context) (int, error) {
	out, err := d.ecs.DescribeServicesWithContext(ctx, d.DescribeServicesInput())
	if err != nil {
		return 0, err
	}
	if len(out.Services) == 0 {
		return 0, nil
	}
	s := out.Services[0]
	for _, dep := range s.Deployments {
		d.Log(formatDeployment(dep))
	}
	return len(s.Deployments), nil
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
	if err := d.LoadTaskDefinition(d.config.TaskDefinitionPath); err != nil {
		return errors.Wrap(err, "create failed")
	}
	svd, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "create failed")
	}
	if *opt.DryRun {
		d.Log("task definition:", d.TaskDefinition.String())
		d.Log("service definition:", svd.String())
		d.Log("DRY RUN OK")
		return nil
	}

	if err := d.RegisterTaskDefinition(ctx); err != nil {
		return errors.Wrap(err, "create failed")
	}
	svd.TaskDefinition = d.Registered.TaskDefinitionArn

	if _, err := d.ecs.CreateServiceWithContext(ctx, svd); err != nil {
		return errors.Wrap(err, "create failed")
	}
	d.Log("Service is created")

	if err := d.WaitServiceStable(ctx); err != nil {
		return errors.Wrap(err, "create failed")
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) Deploy(opt DeployOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting deploy")
	if _, err := d.DescribeServiceStatus(ctx, 0); err != nil {
		return errors.Wrap(err, "deploy failed")
	}
	if err := d.LoadTaskDefinition(d.config.TaskDefinitionPath); err != nil {
		return errors.Wrap(err, "deploy failed")
	}
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	if err := d.RegisterTaskDefinition(ctx); err != nil {
		return errors.Wrap(err, "deploy failed")
	}
	if err := d.UpdateService(ctx, *d.Registered.TaskDefinitionArn); err != nil {
		return errors.Wrap(err, "deploy failed")
	}
	if err := d.WaitServiceStable(ctx); err != nil {
		return errors.Wrap(err, "deploy failed")
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) Rollback(opt RollbackOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting rollback")
	service, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "rollback failed")
	}
	targetArn, err := d.FindRollbackTarget(ctx, *service.TaskDefinition)
	if err != nil {
		return errors.Wrap(err, "rollback failed")
	}
	d.Log("Rollbacking to", arnToName(targetArn))
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	if err := d.UpdateService(ctx, targetArn); err != nil {
		return errors.Wrap(err, "rollback failed")
	}
	if err := d.WaitServiceStable(ctx); err != nil {
		return errors.Wrap(err, "rollback failed")
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

func (d *App) WaitServiceStable(ctx context.Context) error {
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
				if isTerminal && lines > 0 {
					fmt.Print(aec.Up(uint(lines)))
				}
				lines, _ = d.DescribeServiceDeployments(waitCtx)
			}
		}
	}()

	return d.ecs.WaitUntilServicesStableWithContext(ctx, d.DescribeServicesInput())
}

func (d *App) UpdateService(ctx context.Context, taskDefinitionArn string) error {
	d.Log("Updating service...")

	_, err := d.ecs.UpdateServiceWithContext(
		ctx,
		&ecs.UpdateServiceInput{
			Service:        aws.String(d.Service),
			Cluster:        aws.String(d.Cluster),
			TaskDefinition: aws.String(taskDefinitionArn),
		},
	)
	return err
}

func (d *App) RegisterTaskDefinition(ctx context.Context) error {
	d.Log("Registering a new task definition...")

	out, err := d.ecs.RegisterTaskDefinitionWithContext(
		ctx,
		&ecs.RegisterTaskDefinitionInput{
			Family:               d.TaskDefinition.Family,
			TaskRoleArn:          d.TaskDefinition.TaskRoleArn,
			NetworkMode:          d.TaskDefinition.NetworkMode,
			Volumes:              d.TaskDefinition.Volumes,
			PlacementConstraints: d.TaskDefinition.PlacementConstraints,
			ContainerDefinitions: d.TaskDefinition.ContainerDefinitions,
		},
	)
	if err != nil {
		return err
	}
	d.Log("Task definition is registered", taskDefinitionName(out.TaskDefinition))
	d.Registered = out.TaskDefinition
	return nil
}

func (d *App) LoadTaskDefinition(path string) error {
	d.Log("Creating a new task definition by", path)
	c := struct {
		TaskDefinition *ecs.TaskDefinition
	}{}
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return err
	}
	d.TaskDefinition = c.TaskDefinition
	return nil
}

func (d *App) LoadServiceDefinition(path string) (*ecs.CreateServiceInput, error) {
	c := ServiceDefinition{}
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return nil, err
	}
	return &ecs.CreateServiceInput{
		Cluster:                 aws.String(d.config.Cluster),
		DesiredCount:            aws.Int64(1),
		ServiceName:             aws.String(d.config.Service),
		DeploymentConfiguration: c.DeploymentConfiguration,
		LoadBalancers:           c.LoadBalancers,
		PlacementConstraints:    c.PlacementConstraints,
		Role:                    c.Role,
	}, nil
}
