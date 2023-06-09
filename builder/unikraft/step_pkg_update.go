package unikraft

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPkgUpdate struct {
}

// Run executes the step of updating the sources for a package by calling the `kraft pkg update` command.
// This step will not do anything if no source is specified.
func (s *StepPkgUpdate) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	_, ok := state.Get("config").(*Config)
	if !ok {
		err := fmt.Errorf("error encountered obtaining kraft config")
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	driver := state.Get("driver").(Driver)

	err := driver.Update()
	if err != nil {
		err := fmt.Errorf("error encountered updating kraft references: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

// Cleanup should clean up any updated manifest resources.
func (s *StepPkgUpdate) Cleanup(_ multistep.StateBag) {
	// Delete the default manifest folder.
	// The path to the config file can be custom, thus also the manifest folder.
	// For now this can stay empty.
}
