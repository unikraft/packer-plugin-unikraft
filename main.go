package main

import (
	"fmt"
	"os"
	unikraftBuilder "packer-plugin-unikraft/builder/unikraft"
	unikraftPP "packer-plugin-unikraft/post-processor/unikraft"
	unikraftVersion "packer-plugin-unikraft/version"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("builder", new(unikraftBuilder.Builder))
	pps.RegisterPostProcessor("post-processor", new(unikraftPP.PostProcessor))
	pps.SetVersion(unikraftVersion.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
