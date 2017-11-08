package ecspresso

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/kayac/go-config"
)

func taskDefinitionName(t *ecs.TaskDefinition) string {
	return fmt.Sprintf("%s:%d", *t.Family, *t.Revision)
}

type App struct {
	ecs            *ecs.ECS
	Service        string
	Cluster        string
	TaskDefinition *ecs.TaskDefinition
	Registered     *ecs.TaskDefinition
}

func (d *App) DescribeServicesInput() *ecs.DescribeServicesInput {
	return &ecs.DescribeServicesInput{
		Cluster:  aws.String(d.Cluster),
		Services: []*string{aws.String(d.Service)},
	}
}

func (d *App) DescribeServiceDeployments(ctx context.Context) error {
	out, err := d.ecs.DescribeServicesWithContext(ctx, d.DescribeServicesInput())
	if err != nil {
		return err
	}
	if len(out.Services) > 0 {
		for _, dep := range out.Services[0].Deployments {
			d.Log(formatDeployment(dep))
		}
	}
	return nil
}

func Run(conf *Config) error {
	var cancel context.CancelFunc
	ctx := context.Background()
	if conf.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, conf.Timeout)
		defer cancel()
	}

	sess := session.Must(session.NewSession(
		&aws.Config{Region: aws.String(conf.Region)},
	))
	d := &App{
		Service: conf.Service,
		Cluster: conf.Cluster,
		ecs:     ecs.New(sess),
	}
	d.Log("Starting ecspresso")

	if err := d.DescribeServiceDeployments(ctx); err != nil {
		return err
	}
	if err := d.LoadTaskDefinition(conf.TaskDefinitionPath); err != nil {
		return err
	}
	if err := d.RegisterTaskDefinition(ctx); err != nil {
		return err
	}
	if err := d.UpdateService(ctx); err != nil {
		return err
	}
	if err := d.WaitServiceStable(ctx); err != nil {
		return err
	}

	d.Log("Service is stable now. Completed!")
	return nil
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
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick:
				d.DescribeServiceDeployments(waitCtx)
			}
		}
	}()

	return d.ecs.WaitUntilServicesStableWithContext(ctx, d.DescribeServicesInput())
}

func (d *App) UpdateService(ctx context.Context) error {
	d.Log("Updating service...")

	_, err := d.ecs.UpdateServiceWithContext(
		ctx,
		&ecs.UpdateServiceInput{
			Service:        aws.String(d.Service),
			Cluster:        aws.String(d.Cluster),
			TaskDefinition: d.Registered.TaskDefinitionArn,
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
