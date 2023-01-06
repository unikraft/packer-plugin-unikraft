package kraft

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepBuild struct {
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

	return multistep.ActionContinue
}

// Cleanup reverts the changes from the build step.
// In this case, it should remove all build objects, apart from the resulting images.
func (s *StepBuild) Cleanup(_ multistep.StateBag) {
	// TODO move the resulting binary and call `kraft properclean`
}
