package ecspresso

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/fujiwara/ecsta"
	"github.com/pkg/errors"
)

type ExecOption struct {
	ID        *string
	Command   *string
	Container *string

	PortForward *bool
	LocalPort   *int
	Port        *int
}

func (d *App) NewEcsta(ctx context.Context) (*ecsta.Ecsta, error) {
	app, err := ecsta.New(ctx, d.config.Region, d.config.Cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ecsta")
	}
	if fc := d.config.FilterCommand; fc != "" {
		app.Config.Set("filter_command", fc)
	}
	return app, nil
}

func (d *App) Exec(opt ExecOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	ecstaApp, err := d.NewEcsta(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create ecsta app")
	}

	if aws.ToBool(opt.PortForward) {
		return ecstaApp.RunPortforward(ctx, &ecsta.PortforwardOption{
			ID:         aws.ToString(opt.ID),
			Container:  aws.ToString(opt.Container),
			LocalPort:  aws.ToInt(opt.LocalPort),
			RemotePort: aws.ToInt(opt.Port),
			RemoteHost: "", // TODO
		})
	} else {
		return ecstaApp.RunExec(ctx, &ecsta.ExecOption{
			ID:        aws.ToString(opt.ID),
			Command:   aws.ToString(opt.Command),
			Container: aws.ToString(opt.Container),
		})
	}
}
