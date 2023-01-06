//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package kraft

import (
	"context"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
)

const BuilderId = "kraft.builder"

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Prepare(raws ...interface{}) (generatedVars []string, warnings []string, err error) {
	err = config.Decode(&b.config, &config.DecodeOpts{
		PluginType:  "packer.builder.kraft",
		Interpolate: true,
	}, raws...)
	if err != nil {
		return nil, warnings, err
	}
	// Return the placeholder for the generated data that will become available to provisioners and post-processors.
	// If the builder doesn't generate any data, just return an empty slice of string: []string{}
	buildGeneratedData := []string{
		"binaries",
	}
	return buildGeneratedData, warnings, nil
}

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	driver := &KraftDriver{
		Ctx:            &b.config.ctx,
		Ui:             ui,
		CommandContext: KraftCommandContext(),
	}

	steps := []multistep.Step{
		&StepPkgSource{},
		&StepPkgUpdate{},
		&StepPkgPull{},
		&StepSet{},
		&StepBuild{},
		new(commonsteps.StepProvision),
	}

	// Setup the state bag and initial state for the steps
	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)

	state.Put("config", &b.config)
	state.Put("driver", driver)

	// Run!
	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	if b.runner == nil {
		return nil, nil
	}
	b.runner.Run(ctx, state)

	// If there was an error, return that
	if err, ok := state.GetOk("error"); ok {
		return nil, err.(error)
	}

	artifact := &Artifact{
		StateData: map[string]interface{}{
			"binaries": state.Get("binaries"),
		},
	}
	return artifact, nil
}
