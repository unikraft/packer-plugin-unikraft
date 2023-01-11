package unikraft

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepBuild struct {
	resultingBinariesPath []string
}

// Run should execute the purpose of this step
func (s *StepBuild) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	driver := state.Get("driver").(Driver)

	err := driver.Build(config.Path, config.Architecture, config.Platform)
	if err != nil {
		err := fmt.Errorf("error encountered building kraft package: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	// Copy all executable files in the `path/build` folder and move them to `path/dist`
	// Open the folder for reading
	var executableFiles []string = []string{}
	filepath.Walk(filepath.Join(config.Path, "build"), func(path string, info os.FileInfo, err error) error {
		// Check if the file is executable and not a symlink or directory
		if !info.IsDir() && info.Mode()&0111 != 0 && info.Mode()&os.ModeSymlink == 0 {
			// Check if the file is in the root of the build folder
			if !strings.ContainsRune(strings.TrimPrefix(path, filepath.Join(config.Path, "build"))[1:], filepath.Separator) {
				executableFiles = append(executableFiles, path)
			}
		}

		return nil
	})

	// Create the dist folder if it doesn't exist
	if _, err := os.Stat(filepath.Join(config.Path, "dist")); os.IsNotExist(err) {
		os.Mkdir(filepath.Join(config.Path, "dist"), 0755)
	}

	// Move the files to the dist folder
	for _, file := range executableFiles {
		err := os.Rename(file, filepath.Join(config.Path, "dist", filepath.Base(file)))
		if err != nil {
			err := fmt.Errorf("error encountered saving kraft package: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	s.resultingBinariesPath = executableFiles
	state.Put("binaries", s.resultingBinariesPath)

	return multistep.ActionContinue
}

// Cleanup reverts the changes from the build step.
// In this case, it should remove all build objects, apart from the resulting images.
func (s *StepBuild) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	driver := state.Get("driver").(Driver)

	err := driver.ProperClean(config.Path)
	if err != nil {
		err := fmt.Errorf("error encountered cleaning kraft package: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	// Rename the dist folder to build
	err = os.Rename(filepath.Join(config.Path, "dist"), filepath.Join(config.Path, "build"))
	if err != nil {
		err := fmt.Errorf("error encountered cleaning kraft package: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
	}
}
