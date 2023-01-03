package main

import (
	"fmt"
	"os"
	"packer-plugin-scaffolding/builder/kraft"
	scaffoldingData "packer-plugin-scaffolding/datasource/kraft"
	scaffoldingPP "packer-plugin-scaffolding/post-processor/kraft"
	scaffoldingProv "packer-plugin-scaffolding/provisioner/kraft"
	scaffoldingVersion "packer-plugin-scaffolding/version"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("my-builder", new(kraft.Builder))
	pps.RegisterProvisioner("my-provisioner", new(scaffoldingProv.Provisioner))
	pps.RegisterPostProcessor("my-post-processor", new(scaffoldingPP.PostProcessor))
	pps.RegisterDatasource("my-datasource", new(scaffoldingData.Datasource))
	pps.SetVersion(scaffoldingVersion.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
