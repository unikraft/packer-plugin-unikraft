package kraft

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPkgPull struct {
}

// Run should execute the purpose of this step
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

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepPkgPull) Cleanup(_ multistep.StateBag) {
	// Nothing to clean
}
