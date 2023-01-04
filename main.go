package main

import (
	"fmt"
	"os"
	"packer-plugin-kraft/builder/kraft"
	kraftData "packer-plugin-kraft/datasource/kraft"
	kraftPP "packer-plugin-kraft/post-processor/kraft"
	kraftProv "packer-plugin-kraft/provisioner/kraft"
	kraftVersion "packer-plugin-kraft/version"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("builder", new(kraft.Builder))
	pps.RegisterProvisioner("provisioner", new(kraftProv.Provisioner))
	pps.RegisterPostProcessor("post-processor", new(kraftPP.PostProcessor))
	pps.RegisterDatasource("datasource", new(kraftData.Datasource))
	pps.SetVersion(kraftVersion.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
