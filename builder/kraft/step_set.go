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

// Run calls `kraft set` to set specific symbols not found in the configuration file.
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

// Cleanup unsets the symbols set in the Run step.
// Setting and unsetting symbols might create unexpected results.
func (s *StepSet) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	driver := state.Get("driver").(Driver)

	options := make([]string, 0)
	// Split config.Options on ' '
	for _, option := range strings.Split(config.Options, " ") {
		// Split option on '='
		splitOption := strings.Split(option, "=")
		// If option is not a key=value pair, skip it
		if len(splitOption) != 2 {
			continue
		}

		options = append(options, splitOption[0])
	}

	if len(options) == 0 {
		return
	}

	err := driver.Unset(options)
	if err != nil {
		err := fmt.Errorf("error encountered setting symbols: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
	}
}
