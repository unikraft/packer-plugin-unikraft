//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package unikraftpprocessor

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/mitchellh/mapstructure"
)

const BuilderId = "packer.post-processor.unikraft"

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	// The path to the unformatted files.
	FileSource string `mapstructure:"source" required:"true"`
	// The path to the formatted files.
	FileDestination string `mapstructure:"destination" required:"true"`
	// The architecture of the unikernel. This is required.
	Architecture string `mapstructure:"architecture" required:"true"`
	// The platform of the unikernel. This is required.
	Platform string `mapstructure:"platform" required:"true"`
	// The specific target to package.
	Target string `mapstructure:"target"`
	// Whether to push the package to a registry.
	Push bool `mapstructure:"push"`
	// The rootfs to use.
	Rootfs string `mapstructure:"rootfs"`
	// Log level to use.
	LogLevel string `mapstructure:"log_level"`

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
	if c.FileSource == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("file source must be specified"))
	}

	if c.FileDestination == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("file destination must be specified"))
	}

	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	return nil, nil
}
