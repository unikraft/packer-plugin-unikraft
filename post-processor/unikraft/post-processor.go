package unikraftpprocessor

import (
	"context"
	"fmt"
	unikraft "packer-plugin-unikraft/builder/unikraft"

	"github.com/hashicorp/hcl/v2/hcldec"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/mitchellh/mapstructure"
)

type PostProcessor struct {
	config Config
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec { return p.config.FlatMapstructure().HCL2Spec() }

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         "packer.post-processor.unikraft",
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}
	return nil
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, source packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	switch source.BuilderId() {
	case unikraft.BuilderId, BuilderId:
		break
	default:
		return nil, false, false, fmt.Errorf("unknown artifact %s", source.BuilderId())
	}

	var binaries []string
	if err := mapstructure.Decode(source.State("binaries"), &binaries); err != nil {
		err := fmt.Errorf("failed to decode binaries")
		ui.Error(err.Error())
		return source, false, false, err
	}

	driver := &unikraft.KraftDriver{
		Ctx:            &p.config.ctx,
		Ui:             ui,
		CommandContext: unikraft.KraftCommandContext(ui, p.config.LogLevel),
	}

	if p.config.Target != "" {
		p.config.Architecture = ""
		p.config.Platform = ""
	}

	err := driver.Pkg(
		p.config.Architecture,
		p.config.Platform,
		p.config.Target,
		p.config.FileDestination,
		p.config.FileSource,
		p.config.Rootfs,
		p.config.Push,
	)
	if err != nil {
		return nil, false, false, fmt.Errorf("packaging error: %s", err)
	}

	artifact := &unikraft.Artifact{
		StateData: map[string]interface{}{
			"oci": p.config.FileDestination,
		},
	}
	return artifact, true, true, nil
}
