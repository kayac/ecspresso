package ecspresso

import (
	"context"
	"fmt"

	"github.com/fujiwara/ecsta"
)

type ExecOption struct {
	ID        string `help:"task ID" default:""`
	Container string `help:"container name" default:""`

	Run         *ExecRunOption         `cmd:"" default:"withargs" help:"execute command on task"`
	Portforward *ExecPortforwardOption `cmd:"" help:"port forwarding to a task"`
	Cp          *ExecCpOption          `cmd:"" help:"copy files between local and task"`
}

// ExecRunOption is the default subcommand for exec.
type ExecRunOption struct {
	Command string `help:"command to execute" default:"sh"`
}

type ExecPortforwardOption struct {
	LocalPort int    `help:"local port number" default:"0"`
	Port      int    `help:"remote port number" default:"0"`
	Host      string `help:"remote host" default:""`
	L         string `name:"L" short:"L" help:"short expression of local-port:host:port" default:""`
}

type ExecCpOption struct {
	Src      string `arg:"" help:"source"`
	Dest     string `arg:"" help:"destination"`
	Port     int    `help:"port number for file transfer" default:"12345"`
	Progress bool   `help:"show progress bar" default:"true" negatable:""`
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

	switch {
	case opt.Cp != nil:
		return ecstaApp.RunCp(ctx, &ecsta.CpOption{
			Src:       opt.Cp.Src,
			Dest:      opt.Cp.Dest,
			Port:      opt.Cp.Port,
			Progress:  opt.Cp.Progress,
			ID:        opt.ID,
			Container: opt.Container,
			Family:    &family,
			Service:   service,
		})
	case opt.Portforward != nil:
		return ecstaApp.RunPortforward(ctx, &ecsta.PortforwardOption{
			ID:         opt.ID,
			Container:  opt.Container,
			LocalPort:  opt.Portforward.LocalPort,
			RemotePort: opt.Portforward.Port,
			RemoteHost: opt.Portforward.Host,
			L:          opt.Portforward.L,
			Family:     &family,
			Service:    service,
		})
	case opt.Run != nil:
		return ecstaApp.RunExec(ctx, &ecsta.ExecOption{
			ID:        opt.ID,
			Command:   opt.Run.Command,
			Container: opt.Container,
			Family:    &family,
			Service:   service,
		})
	}
	return nil
}
