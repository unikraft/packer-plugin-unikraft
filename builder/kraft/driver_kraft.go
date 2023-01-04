package kraft

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"sync"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

// TODO: Add implementation + call to each kraft command
type KraftDriver struct {
	Ui  packersdk.Ui
	Ctx *interpolate.Context

	// The directory Docker should use to store its client configuration.
	// Provides an isolated client configuration to each Docker operation to
	// prevent race conditions.
	ConfigDir string

	l sync.Mutex
}

func (d *KraftDriver) Build(path, architecture, platform string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("kraft", "build", "-m", architecture, "-p", platform, path)
	cmd.Stderr = &stderr

	log.Printf("Building image: %s", path)
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("error building image: %s\nStderr: %s",
			err, stderr.String())
		return err
	}

	return nil
}

func (d *KraftDriver) Pull(source, workdir string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("kraft", "pkg", "pull", "-w", workdir, source)
	cmd.Stderr = &stderr

	log.Printf("Pulling image: %s", source)
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("error pulling image: %s\nStderr: %s",
			err, stderr.String())
		return err
	}

	return nil
}

func (d *KraftDriver) Source(source string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("kraft", "pkg", "source", source)
	cmd.Stderr = &stderr

	log.Printf("Sourcing image: %s", source)
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("error sourcing image: %s\nStderr: %s",
			err, stderr.String())
		return err
	}
	return nil
}

func (d *KraftDriver) Update() error {
	var stderr bytes.Buffer
	cmd := exec.Command("kraft", "pkg", "update")
	cmd.Stderr = &stderr

	log.Printf("Updating sources")
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("error updating sources: %s\nStderr: %s",
			err, stderr.String())
		return err
	}
	return nil
}

func (d *KraftDriver) Set(options map[string]string) error {
	var strOptions []string = []string{"set"}
	var stderr bytes.Buffer

	for key, value := range options {
		strOptions = append(strOptions, fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.Command("kraft", strOptions...)
	cmd.Stderr = &stderr

	log.Printf("Setting custom symbols image")
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("error setting symbols: %s\nStderr: %s",
			err, stderr.String())
		return err
	}
	return nil
}
