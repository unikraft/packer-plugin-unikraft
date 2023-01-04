package kraft

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepSet struct {
}

// Run should execute the purpose of this step
func (s *StepSet) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	driver := state.Get("driver").(Driver)

	options := make(map[string]string)
	// Split config.Options on ' '
	for _, option := range strings.Split(config.Options, " ") {
		// Split option on '='
		splitOption := strings.Split(option, "=")
		// If option is not a key=value pair, skip it
		if len(splitOption) != 2 {
			continue
		}
		// Add option to options map
		options[splitOption[0]] = splitOption[1]
	}

	if len(options) == 0 {
		return multistep.ActionContinue
	}
	err := driver.Set(options)
	if err != nil {
		err := fmt.Errorf("error encountered setting symbols: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepSet) Cleanup(_ multistep.StateBag) {
	// Nothing to clean
}
