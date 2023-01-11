//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package unikraftdata

import (
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type Config struct {
	ctx interpolate.Context
}

type DatasourceOutput struct {
	Map map[string]string `mapstructure:"map"`
}
