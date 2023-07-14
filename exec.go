package ecspresso

import (
	"context"
	"fmt"

	"github.com/fujiwara/ecsta"
)

type ExecOption struct {
	ID        string `help:"task ID" default:""`
	Command   string `help:"command to execute" default:"sh"`
	Container string `help:"container name" default:""`

	PortForward bool   `help:"enable port forward" default:"false"`
	LocalPort   int    `help:"local port number" default:"0"`
	Port        int    `help:"remote port number (required for --port-forward)" default:"0"`
	Host        string `help:"remote host (required for --port-forward)" default:""`
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
	// Do not call d.Start() because timeout disabled for exec.

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
			ID:         opt.ID,
			Container:  opt.Container,
			LocalPort:  opt.LocalPort,
			RemotePort: opt.Port,
			RemoteHost: opt.Host,
			Family:     &family,
			Service:    service,
		})
	} else {
		return ecstaApp.RunExec(ctx, &ecsta.ExecOption{
			ID:        opt.ID,
			Command:   opt.Command,
			Container: opt.Container,
			Family:    &family,
			Service:   service,
		})
	}
}
