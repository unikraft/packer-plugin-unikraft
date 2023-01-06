package kraft

import (
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/spf13/cobra"

	kraftbuild "kraftkit.sh/cmd/kraft/build"
	kraftpull "kraftkit.sh/cmd/kraft/pkg/pull"
	kraftsource "kraftkit.sh/cmd/kraft/pkg/source"
	kraftupdate "kraftkit.sh/cmd/kraft/pkg/update"
	kraftproperclean "kraftkit.sh/cmd/kraft/properclean"
	kraftset "kraftkit.sh/cmd/kraft/set"
	kraftunset "kraftkit.sh/cmd/kraft/unset"
	// kraftunsource "kraftkit.sh/cmd/kraft/pkg/unsource"
)

type KraftDriver struct {
	Ui  packersdk.Ui
	Ctx *interpolate.Context

	CommandContext *cobra.Command
}

func (d *KraftDriver) Build(path, architecture, platform string) error {
	build := kraftbuild.Build{
		Architecture: architecture,
		Platform:     platform,
	}
	return build.Run(d.CommandContext, []string{path})
}

func (d *KraftDriver) ProperClean(path string) error {
	properclean := kraftproperclean.ProperClean{}

	return properclean.Run(d.CommandContext, []string{path})
}

func (d *KraftDriver) Pull(source, workdir string) error {
	pull := kraftpull.Pull{
		Workdir: workdir,
	}

	return pull.Run(d.CommandContext, []string{source})
}

func (d *KraftDriver) Source(source string) error {
	src := kraftsource.Source{}

	return src.Run(d.CommandContext, []string{source})
}

func (d *KraftDriver) Unsource(source string) error {
	// src := kraftunsource.Unsource{}

	// return src.Run(d.CommandContext, []string{source})
	return nil
}

func (d *KraftDriver) Update() error {
	update := kraftupdate.Update{
		Manager: "manifest",
	}

	return update.Run(d.CommandContext, []string{})
}

func (d *KraftDriver) Set(options map[string]string) error {
	set := kraftset.Set{}
	opts := []string{}

	for k, v := range options {
		opts = append(opts, fmt.Sprintf("%s=%s", k, v))
	}

	return set.Run(d.CommandContext, opts)
}

func (d *KraftDriver) Unset(options []string) error {
	unset := kraftunset.Unset{}

	return unset.Run(d.CommandContext, options)
}
