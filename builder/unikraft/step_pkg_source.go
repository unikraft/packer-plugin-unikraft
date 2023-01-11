package unikraft

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPkgSource struct {
}

// Run executes the step of sourcing a package by calling the `kraft pkg source` command.
// This step is skipped if no source is specified.
func (s *StepPkgSource) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if config.SourceSource == "" {
		return multistep.ActionContinue
	}

	driver := state.Get("driver").(Driver)

	err := driver.Source(config.SourceSource)
	if err != nil {
		err := fmt.Errorf("error encountered sourcing kraft package: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

// Cleanup is called after the step is finished and calls `kraft pkg unsource` to remove the source.
func (s *StepPkgSource) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	config, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	if config.SourceSource == "" {
		return
	}

	driver := state.Get("driver").(Driver)

	err := driver.Unsource(config.SourceSource)
	if err != nil {
		err := fmt.Errorf("error encountered unsourcing kraft package: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}
}
