package unikraftpprocessor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	unikraft "packer-plugin-unikraft/builder/unikraft"

	"github.com/hashicorp/hcl/v2/hcldec"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/mitchellh/mapstructure"
)

type PostProcessor struct {
	config Config
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec { return p.config.FlatMapstructure().HCL2Spec() }

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         "packer.post-processor.unikraft",
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}
	return nil
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, source packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	switch source.BuilderId() {
	case unikraft.BuilderId, BuilderId:
		break
	default:
		return nil, false, false, fmt.Errorf("unknown artifact %s", source.BuilderId())
	}

	var binaries []string
	if err := mapstructure.Decode(source.State("binaries"), &binaries); err != nil {
		err := fmt.Errorf("failed to decode binaries")
		ui.Error(err.Error())
		return source, false, false, err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	args := []string{p.config.FileSource, "-depth", "-print"}

	cmd := exec.Command("find", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, false, false, err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("error listing files: %s\nStderr: %s",
			err, stderr.String())
		return nil, false, false, err
	}

	var stdout2 bytes.Buffer
	var stderr2 bytes.Buffer

	// Pipe the output to the archiver
	cmd2 := exec.Command("bsdcpio", []string{"-o", "--format", "newc"}...)
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	cmd2.Stdin = &stdout

	if err := cmd2.Start(); err != nil {
		return nil, false, false, err
	}

	if err := cmd2.Wait(); err != nil {
		err = fmt.Errorf("error archiving files: %s\nStderr: %s",
			err, stderr.String())
		return nil, false, false, err
	}

	// Write the output from inside stdout2 into a file
	if err := ioutil.WriteFile(p.config.FileDestination, stdout2.Bytes(), 0644); err != nil {
		return nil, false, false, err
	}

	// Update the artifact with the new file
	artifact := &unikraft.Artifact{
		StateData: map[string]interface{}{
			"initramfs": p.config.FileDestination,
		},
	}
	return artifact, true, true, nil
}
