package ecspresso

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/fujiwara/ecsta"
)

type ExecOption struct {
	ID        *string `help:"task ID" default:""`
	Command   *string `help:"command to execute" default:"sh"`
	Container *string `help:"container name" default:""`

	PortForward bool    `help:"enable port forward" default:"false"`
	LocalPort   *int    `help:"local port number" default:"0"`
	Port        *int    `help:"remote port number (required for --port-forward)" default:"0"`
	Host        *string `help:"remote host (required for --port-forward)" default:""`
}

func (d *App) NewEcsta(ctx context.Context) (*ecsta.Ecsta, error) {
	app, err := ecsta.New(ctx, d.config.Region, d.config.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to create ecsta application: %w", err)
	}
	if fc := d.FilterCommand(); fc != "" {
		app.Config.Set("filter_command", fc)
	}
	return app, nil
}

func (d *App) Exec(ctx context.Context, opt ExecOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	ecstaApp, err := d.NewEcsta(ctx)
	if err != nil {
		return err
	}
	family, err := d.taskDefinitionFamily(ctx)
	if err != nil {
		return err
	}
	var service *string
	if d.config.Service != "" {
		service = &d.config.Service
	}

	if opt.PortForward {
		return ecstaApp.RunPortforward(ctx, &ecsta.PortforwardOption{
			ID:         aws.ToString(opt.ID),
			Container:  aws.ToString(opt.Container),
			LocalPort:  aws.ToInt(opt.LocalPort),
			RemotePort: aws.ToInt(opt.Port),
			RemoteHost: aws.ToString(opt.Host),
			Family:     &family,
			Service:    service,
		})
	} else {
		return ecstaApp.RunExec(ctx, &ecsta.ExecOption{
			ID:        aws.ToString(opt.ID),
			Command:   aws.ToString(opt.Command),
			Container: aws.ToString(opt.Container),
			Family:    &family,
			Service:   service,
		})
	}
}
