package unikraft

import (
	"context"
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type KraftDriver struct {
	Ui  packersdk.Ui
	Ctx *interpolate.Context

	CommandContext context.Context
}

func (d *KraftDriver) Build(path, architecture, platform string) error {
	c := Build{
		Architecture: architecture,
		Platform:     platform,
		Jobs:         4,
	}
	return c.BuildCmd(d.CommandContext, []string{path})
}

func (d *KraftDriver) ProperClean(path string) error {
	c := ProperClean{}

	return c.ProperCleanCmd(d.CommandContext, []string{path})
}

func (d *KraftDriver) Pull(source, workdir string) error {
	c := Pull{
		Workdir: workdir,
	}

	return c.PullCmd(d.CommandContext, []string{source})
}

func (d *KraftDriver) Source(source string) error {
	c := Source{}

	return c.SourceCmd(d.CommandContext, []string{source})
}

func (d *KraftDriver) Unsource(source string) error {
	c := Unsource{}

	return c.UnsourceCmd(d.CommandContext, []string{source})
}

func (d *KraftDriver) Update() error {
	c := Update{
		Manager: "manifest",
	}

	return c.UpdateCmd(d.CommandContext, []string{})
}

func (d *KraftDriver) Set(options map[string]string) error {
	c := Set{}
	opts := []string{}

	for k, v := range options {
		opts = append(opts, fmt.Sprintf("%s=%s", k, v))
	}

	return c.SetCmd(d.CommandContext, opts)
}
