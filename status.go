package ecspresso

import "context"

type StatusOption struct {
	Events int `help:"show events num" default:"10"`
}

func (d *App) Status(ctx context.Context, opt StatusOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()
	_, err := d.DescribeServiceStatus(ctx, opt.Events)
	return err
}
