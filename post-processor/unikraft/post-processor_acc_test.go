package unikraftpprocessor

import (
	_ "embed"
	"fmt"
	"os/exec"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/acctest"
)

//go:embed test-fixtures/template.pkr.hcl
var testPostProcessorHCL2Basic string

// Run with: PACKER_ACC=1 go test -count 1 -v ./post-processor/scaffolding/post-processor_acc_test.go  -timeout=120m
func TestAccScaffoldingPostProcessor(t *testing.T) {
	testCase := &acctest.PluginTestCase{
		Name: "unikraft_post-processor_basic_test",
		Setup: func() error {
			return nil
		},
		Teardown: func() error {
			return nil
		},
		Template: testPostProcessorHCL2Basic,
		Type:     "unikraft-post-processor",
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if buildCommand.ProcessState != nil {
				if buildCommand.ProcessState.ExitCode() != 0 {
					return fmt.Errorf("bad exit code. Logfile: %s", logfile)
				}
			}

			return nil
		},
	}
	acctest.TestPlugin(t, testCase)
}
