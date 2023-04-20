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

	// The type of the file system to create.
	Type string `mapstructure:"type"`
	// The path to the unformatted files.
	FileSource string `mapstructure:"source"`
	// The path to the formatted files.
	FileDestination string `mapstructure:"destination"`
	// The architecture of the unikernel.
	Architecture string `mapstructure:"architecture"`
	// The platform of the unikernel.
	Platform string `mapstructure:"platform"`

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

	if c.Type == "" || (c.Type != "cpio" && c.Type != "oci") {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("package type must be specified"))
	}

	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	return nil, nil
}
