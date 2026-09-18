// Package skillscmd provides Kong-compatible command structs for skillsmith
// integration. Embed [Commands] in a Kong CLI options struct to add a
// "skills" subcommand.
//
// Usage:
//
//	type CLIOptions struct {
//	    Skills *skillscmd.Commands `cmd:"" help:"manage agent skills"`
//	}
//
// After Kong parsing, dispatch with:
//
//	smith, _ := skillsmith.New("mytool", version, skillsFS)
//	err := opts.Skills.Dispatch(ctx, smith)
package skillscmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Songmu/skillsmith"
)

// Commands defines Kong subcommands for skills management.
type Commands struct {
	List      *ListOption   `cmd:"" help:"list available skills"`
	Install   *ModifyOption `cmd:"" help:"install skills"`
	Update    *ModifyOption `cmd:"" help:"update installed skills"`
	Reinstall *ModifyOption `cmd:"" help:"reinstall all managed skills"`
	Uninstall *ModifyOption `cmd:"" help:"uninstall managed skills"`
	Status    *StatusOption `cmd:"" help:"show installation status"`
}

// ListOption defines CLI options for the list subcommand.
type ListOption struct{}

// ModifyOption defines CLI options for install/update/reinstall/uninstall subcommands.
type ModifyOption struct {
	Scope  string `help:"install scope (user or repo)" default:"" enum:",user,repo"`
	Prefix string `help:"override install directory" default:""`
	DryRun bool   `help:"preview changes without applying" name:"dry-run" default:"false"`
	Force  bool   `help:"overwrite unmanaged skills or force downgrade" default:"false"`
}

func (o *ModifyOption) options() skillsmith.Options {
	return skillsmith.Options{
		Scope:  o.Scope,
		Prefix: o.Prefix,
		DryRun: o.DryRun,
		Force:  o.Force,
	}
}

// StatusOption defines CLI options for the status subcommand.
type StatusOption struct {
	Scope  string `help:"install scope (user or repo)" default:"" enum:",user,repo"`
	Prefix string `help:"override install directory" default:""`
}

func (o *StatusOption) options() skillsmith.Options {
	return skillsmith.Options{
		Scope:  o.Scope,
		Prefix: o.Prefix,
	}
}

// Dispatch dispatches to the active subcommand based on which option is non-nil.
//
// Do not name this method "Run". Kong treats a Run() method of a command struct specially:
// Kong v1.16+ does not require a subcommand for a struct that has it.
func (c *Commands) Dispatch(ctx context.Context, s *skillsmith.Smith) error {
	switch {
	case c.List != nil:
		return runList(ctx, s)
	case c.Install != nil:
		return runInstall(ctx, s, c.Install.options())
	case c.Update != nil:
		return runUpdate(ctx, s, c.Update.options())
	case c.Reinstall != nil:
		return runReinstall(ctx, s, c.Reinstall.options())
	case c.Uninstall != nil:
		return runUninstall(ctx, s, c.Uninstall.options())
	case c.Status != nil:
		return runStatus(ctx, s, c.Status.options())
	default:
		return fmt.Errorf("unknown skills subcommand")
	}
}

func runList(ctx context.Context, s *skillsmith.Smith) error {
	skills, err := s.List(ctx)
	if err != nil {
		return err
	}
	if len(skills) == 0 {
		fmt.Println("no skills found")
		return nil
	}
	for _, sk := range skills {
		if sk.Description != "" {
			fmt.Printf("%-30s %s\n", sk.Dir, sk.Description)
		} else {
			fmt.Println(sk.Dir)
		}
	}
	return nil
}

func runInstall(ctx context.Context, s *skillsmith.Smith, opts skillsmith.Options) error {
	result, err := s.Install(ctx, opts)
	if err != nil {
		return err
	}
	printCopyResult(result, "installed", opts.DryRun)
	return nil
}

func runUpdate(ctx context.Context, s *skillsmith.Smith, opts skillsmith.Options) error {
	result, err := s.Update(ctx, opts)
	if err != nil {
		return err
	}
	printCopyResult(result, "updated", opts.DryRun)
	return nil
}

func runReinstall(ctx context.Context, s *skillsmith.Smith, opts skillsmith.Options) error {
	result, err := s.Reinstall(ctx, opts)
	if err != nil {
		return err
	}
	printCopyResult(result, "reinstalled", opts.DryRun)
	return nil
}

func runUninstall(ctx context.Context, s *skillsmith.Smith, opts skillsmith.Options) error {
	result, err := s.Uninstall(ctx, opts)
	if err != nil {
		return err
	}
	for _, a := range result.Actions {
		switch a.Action {
		case "uninstalled":
			if opts.DryRun {
				fmt.Printf("uninstalled (dry-run): %s\n", a.Dir)
			} else {
				fmt.Printf("uninstalled: %s\n", a.Dir)
			}
		case "skipped":
			fmt.Printf("skipped:     %s — %s\n", a.Dir, a.Message)
		}
	}
	if opts.DryRun {
		fmt.Println("[dry-run] no changes were made")
	}
	return nil
}

func runStatus(ctx context.Context, s *skillsmith.Smith, opts skillsmith.Options) error {
	result, err := s.Status(ctx, opts)
	if err != nil {
		return err
	}
	for _, ss := range result.Skills {
		switch {
		case !ss.Installed:
			fmt.Printf("%-30s not installed\n", ss.Dir)
		case ss.MetadataError != nil:
			fmt.Printf("%-30s installed (metadata unreadable: %v)\n", ss.Dir, ss.MetadataError)
		case ss.UpToDate:
			fmt.Printf("%-30s installed %s (up to date)\n", ss.Dir, ss.InstalledVersion)
		default:
			fmt.Printf("%-30s installed %s → available %s\n", ss.Dir, ss.InstalledVersion, ss.AvailableVersion)
		}
	}
	return nil
}

func printCopyResult(result *skillsmith.CopyResult, verb string, dryRun bool) {
	for _, a := range result.Actions {
		switch a.Action {
		case verb:
			if dryRun {
				fmt.Printf("%s (dry-run): %s\n", verb, a.Dir)
			} else {
				fmt.Printf("%s: %s\n", verb, a.Dir)
			}
		case "skipped":
			fmt.Printf("skipped:   %s — %s\n", a.Dir, a.Message)
		case "warned":
			fmt.Fprintf(os.Stderr, "warning:   %s — %s\n", a.Dir, a.Message)
		}
	}
	if dryRun {
		fmt.Println("[dry-run] no changes were made")
	}
}
