package kraft

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPkgPull struct {
}

// Run calls `kraft pkg pull` with the given repository to pull it locally.
func (s *StepPkgPull) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if config.PullSource == "" || config.Workdir == "" {
		return multistep.ActionContinue
	}

	driver := state.Get("driver").(Driver)

	err := driver.Pull(config.PullSource, config.Workdir)
	if err != nil {
		err := fmt.Errorf("error encountered pulling kraft package: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

// Cleanup reverts the changes from the pull step.
// In this case, it should remove all downloaded files, apart from the resulting images from the build step.
func (s *StepPkgPull) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	if config.PullSource == "" || config.Workdir == "" {
		return
	}

	// Delete everything in the builddir except the resulting images in the `build` directory.
	buildDir := filepath.Join(config.Workdir, ".unikraft", "apps", config.PullSource)
	files, err := ioutil.ReadDir(buildDir)
	if err != nil {
		err := fmt.Errorf("error encountered reading workdir: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	for _, file := range files {
		// If the file is a directory, and it is not the build directory, delete it.
		if !file.IsDir() || (file.IsDir() && file.Name() != "build" && file.Name() != "fs0") {
			err := os.RemoveAll(filepath.Join(buildDir, file.Name()))
			if err != nil {
				err := fmt.Errorf("error encountered removing directory: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return
			}
		}
	}
}
