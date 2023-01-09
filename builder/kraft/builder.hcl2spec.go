// Code generated by "packer-sdc mapstructure-to-hcl2"; DO NOT EDIT.

package kraft

import (
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"
)

// FlatConfig is an auto-generated flat version of Config.
// Where the contents of a field with a `mapstructure:,squash` tag are bubbled up.
type FlatConfig struct {
	PackerBuildName     *string           `mapstructure:"packer_build_name" cty:"packer_build_name" hcl:"packer_build_name"`
	PackerBuilderType   *string           `mapstructure:"packer_builder_type" cty:"packer_builder_type" hcl:"packer_builder_type"`
	PackerCoreVersion   *string           `mapstructure:"packer_core_version" cty:"packer_core_version" hcl:"packer_core_version"`
	PackerDebug         *bool             `mapstructure:"packer_debug" cty:"packer_debug" hcl:"packer_debug"`
	PackerForce         *bool             `mapstructure:"packer_force" cty:"packer_force" hcl:"packer_force"`
	PackerOnError       *string           `mapstructure:"packer_on_error" cty:"packer_on_error" hcl:"packer_on_error"`
	PackerUserVars      map[string]string `mapstructure:"packer_user_variables" cty:"packer_user_variables" hcl:"packer_user_variables"`
	PackerSensitiveVars []string          `mapstructure:"packer_sensitive_variables" cty:"packer_sensitive_variables" hcl:"packer_sensitive_variables"`
	Architecture        *string           `mapstructure:"architecture" required:"true" cty:"architecture" hcl:"architecture"`
	Platform            *string           `mapstructure:"platform" required:"true" cty:"platform" hcl:"platform"`
	Force               *bool             `mapstructure:"force" cty:"force" hcl:"force"`
	Target              *string           `mapstructure:"target" cty:"target" hcl:"target"`
	Jobs                *int              `mapstructure:"jobs" cty:"jobs" hcl:"jobs"`
	Fast                *bool             `mapstructure:"fast" cty:"fast" hcl:"fast"`
	Path                *string           `mapstructure:"build_path" required:"true" cty:"build_path" hcl:"build_path"`
	PullSource          *string           `mapstructure:"pull_source" cty:"pull_source" hcl:"pull_source"`
	Workdir             *string           `mapstructure:"workdir" cty:"workdir" hcl:"workdir"`
	SourceSource        *string           `mapstructure:"source_source" cty:"source_source" hcl:"source_source"`
	Options             *string           `mapstructure:"options" cty:"options" hcl:"options"`
}

// FlatMapstructure returns a new FlatConfig.
// FlatConfig is an auto-generated flat version of Config.
// Where the contents a fields with a `mapstructure:,squash` tag are bubbled up.
func (*Config) FlatMapstructure() interface{ HCL2Spec() map[string]hcldec.Spec } {
	return new(FlatConfig)
}

// HCL2Spec returns the hcl spec of a Config.
// This spec is used by HCL to read the fields of Config.
// The decoded values from this spec will then be applied to a FlatConfig.
func (*FlatConfig) HCL2Spec() map[string]hcldec.Spec {
	s := map[string]hcldec.Spec{
		"packer_build_name":          &hcldec.AttrSpec{Name: "packer_build_name", Type: cty.String, Required: false},
		"packer_builder_type":        &hcldec.AttrSpec{Name: "packer_builder_type", Type: cty.String, Required: false},
		"packer_core_version":        &hcldec.AttrSpec{Name: "packer_core_version", Type: cty.String, Required: false},
		"packer_debug":               &hcldec.AttrSpec{Name: "packer_debug", Type: cty.Bool, Required: false},
		"packer_force":               &hcldec.AttrSpec{Name: "packer_force", Type: cty.Bool, Required: false},
		"packer_on_error":            &hcldec.AttrSpec{Name: "packer_on_error", Type: cty.String, Required: false},
		"packer_user_variables":      &hcldec.AttrSpec{Name: "packer_user_variables", Type: cty.Map(cty.String), Required: false},
		"packer_sensitive_variables": &hcldec.AttrSpec{Name: "packer_sensitive_variables", Type: cty.List(cty.String), Required: false},
		"architecture":               &hcldec.AttrSpec{Name: "architecture", Type: cty.String, Required: false},
		"platform":                   &hcldec.AttrSpec{Name: "platform", Type: cty.String, Required: false},
		"force":                      &hcldec.AttrSpec{Name: "force", Type: cty.Bool, Required: false},
		"target":                     &hcldec.AttrSpec{Name: "target", Type: cty.String, Required: false},
		"jobs":                       &hcldec.AttrSpec{Name: "jobs", Type: cty.Number, Required: false},
		"fast":                       &hcldec.AttrSpec{Name: "fast", Type: cty.Bool, Required: false},
		"build_path":                 &hcldec.AttrSpec{Name: "build_path", Type: cty.String, Required: false},
		"pull_source":                &hcldec.AttrSpec{Name: "pull_source", Type: cty.String, Required: false},
		"workdir":                    &hcldec.AttrSpec{Name: "workdir", Type: cty.String, Required: false},
		"source_source":              &hcldec.AttrSpec{Name: "source_source", Type: cty.String, Required: false},
		"options":                    &hcldec.AttrSpec{Name: "options", Type: cty.String, Required: false},
	}
	return s
}