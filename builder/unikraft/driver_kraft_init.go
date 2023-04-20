package unikraft

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/clean"
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
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/manifest"
	"kraftkit.sh/oci"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
)

const (
	ManifestContext pack.ContextKey = "manifest"
)

type Kraft struct{}

func (k *Kraft) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("kraft command should not be called")
}

// KraftCommandContext returns a context with the Kraft commands registered.
// It needs to initialise the commands to ensure that internal context functions are called.
func KraftCommandContext() context.Context {
	command := cobra.Command{}
	ctx := signals.SetupSignalContext()

	cfg, err := config.NewDefaultKraftKitConfig()
	if err != nil {
		panic(err)
	}
	cfgm, err := config.NewConfigManager(
		cfg,
		config.WithFile[config.KraftKit](config.DefaultConfigFile(), true),
	)
	if err != nil {
		panic(err)
	}

	ctx = config.WithConfigManager(ctx, cfgm)

	packmanager.RegisterPackageManager(manifest.ManifestFormat, manifest.NewManifestManager)
	packmanager.RegisterPackageManager(oci.OCIFormat, oci.NewOCIManager)
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		panic(err)
	}

	command.SetContext(packmanager.WithPackageManager(ctx, pm))
	cmd := cmdfactory.New(&Kraft{}, command)
	cmd.AddCommand(build.New())
	cmd.AddCommand(clean.New())
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

	cmdfactory.AttributeFlags(cmd, cfgm.Config, os.Args...)

	return cmd.Context()
}
