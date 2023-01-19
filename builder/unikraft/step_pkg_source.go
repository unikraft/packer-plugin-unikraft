package unikraft

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPkgSource struct {
	defaultAlreadyMissing bool
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

	driver := state.Get("driver").(Driver)

	if config.SourcesNoDefault {
		s.defaultAlreadyMissing = false
		err := driver.Source("https://manifests.kraftkit.sh/index.yaml")
		if err != nil {
			// Do not fail if there's no default manifest, but output the error
			s.defaultAlreadyMissing = true
			err := fmt.Errorf("no default package link to unsource, continuing: %s", err)
			ui.Error(err.Error())
		}
	}

	if len(config.Sources) == 0 {
		return multistep.ActionContinue
	}

	for _, source := range config.Sources {
		err := driver.Source(source)
		if err != nil {
			err := fmt.Errorf("error encountered sourcing kraft package: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
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

	driver := state.Get("driver").(Driver)

	if config.SourcesNoDefault || s.defaultAlreadyMissing {
		err := driver.Source("https://manifests.kraftkit.sh/index.yaml")
		if err != nil {
			// Do not fail if there's no default manifest, but output the error
			err := fmt.Errorf("could not resource default manifest, continuing: %s", err)
			ui.Error(err.Error())
		}
	}

	if len(config.Sources) == 0 {
		return
	}

	for _, source := range config.Sources {
		err := driver.Unsource(source)
		if err != nil {
			err := fmt.Errorf("error encountered unsourcing kraft package: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return
		}
	}
}
