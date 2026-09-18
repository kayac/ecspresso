package ecspresso

import (
	"context"

	"github.com/Songmu/skillsmith"
	"github.com/kayac/ecspresso/v2/skillscmd"
)

func dispatchSkills(ctx context.Context, opts *skillscmd.Commands) error {
	version := Version
	if version == "" {
		version = "v0.0.0-dev"
	}
	s, err := skillsmith.New("ecspresso", version, skillsFS)
	if err != nil {
		return err
	}
	return opts.Dispatch(ctx, s)
}
