package kraft

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/clean"
	"kraftkit.sh/cmd/kraft/configure"
	"kraftkit.sh/cmd/kraft/events"
	"kraftkit.sh/cmd/kraft/fetch"
	"kraftkit.sh/cmd/kraft/menu"
	"kraftkit.sh/cmd/kraft/pkg"
	"kraftkit.sh/cmd/kraft/prepare"
	"kraftkit.sh/cmd/kraft/properclean"
	"kraftkit.sh/cmd/kraft/ps"
	"kraftkit.sh/cmd/kraft/rm"
	"kraftkit.sh/cmd/kraft/run"
	"kraftkit.sh/cmd/kraft/set"
	"kraftkit.sh/cmd/kraft/stop"
	"kraftkit.sh/cmd/kraft/unset"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/manifest"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
)

const (
	ManifestContext pack.ContextKey = "manifest"
)

type Kraft struct{}

func (k *Kraft) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func KraftCommandContext() *cobra.Command {
	command := cobra.Command{}
	command.SetContext(context.Background())
	cmd := cmdfactory.New(&Kraft{}, command)
	cmd.AddCommand(build.New())
	cmd.AddCommand(clean.New())
	cmd.AddCommand(configure.New())
	cmd.AddCommand(events.New())
	cmd.AddCommand(fetch.New())
	cmd.AddCommand(menu.New())
	cmd.AddCommand(pkg.New())
	cmd.AddCommand(prepare.New())
	cmd.AddCommand(properclean.New())
	cmd.AddCommand(ps.New())
	cmd.AddCommand(rm.New())
	cmd.AddCommand(run.New())
	cmd.AddCommand(set.New())
	cmd.AddCommand(stop.New())
	cmd.AddCommand(unset.New())
	return cmd
}

func init() {
	options, err := packmanager.NewPackageManagerOptions()
	if err != nil {
		panic(fmt.Sprintf("could not register package manager options: %s", err))
	}

	manager, err := manifest.NewManifestPackageManagerFromOptions(options)
	if err != nil {
		panic(fmt.Sprintf("could not register package manager: %s", err))
	}

	// Register a new pack.Package type
	packmanager.RegisterPackageManager(ManifestContext, manager)
}
