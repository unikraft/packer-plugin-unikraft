//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package unikraft

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/mitchellh/mapstructure"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	// The architecture to build for. This is required.
	Architecture string `mapstructure:"architecture" required:"true"`
	// The platform to build for. This is required.
	Platform string `mapstructure:"platform" required:"true"`
	// Force a rebuild of the image from scratch.
	Force bool `mapstructure:"force"`
	// The name of the image to build.
	Target string `mapstructure:"target"`
	// Max number of jobs to run in parallel.
	Jobs int `mapstructure:"jobs"`
	// Build jobs in parallel.
	Fast bool `mapstructure:"fast"`

	// The path to the build directory. This is required.
	Path string `mapstructure:"build_path" required:"true"`
	// The path to the pull source.
	PullSource string `mapstructure:"pull_source"`
	// The workdir to pull in.
	Workdir string `mapstructure:"workdir"`
	// Links to the sources.
	Sources []string `mapstructure:"sources"`
	// Unsources the default manifest location for using custom sources.
	SourcesNoDefault bool `mapstructure:"sources_no_default"`
	// Set of options to set.
	Options string `mapstructure:"options"`
	// Keep the specification of the build.
	KeepConfig bool `mapstructure:"keep_config"`

	ctx interpolate.Context
}

func (c *Config) Prepare(raws ...interface{}) ([]string, error) {
	var md mapstructure.Metadata
	err := config.Decode(c, &config.DecodeOpts{
		Metadata:           &md,
		PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &c.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"run_command",
			},
		},
	}, raws...)
	if err != nil {
		return nil, err
	}

	// Accumulate any errors
	var errs *packer.MultiError
	if c.Architecture == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("architecture must be specified"))
	}

	if c.Platform == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("platform must be specified"))
	}

	if c.Jobs < 1 {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("jobs must be greater than 0"))
	}

	if c.Path == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("build_path must be specified"))
	}

	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	return nil, nil
}
