package unikraft

import "fmt"

// packersdk.Artifact implementation
type Artifact struct {
	// StateData should store data such as GeneratedData
	// to be shared with post-processors
	StateData map[string]interface{}
}

func (*Artifact) BuilderId() string {
	return BuilderId
}

func (a *Artifact) Files() []string {
	files := append([]string{}, a.StateData["binaries"].([]string)...)
	files = append(files, a.StateData["initramfs"].([]string)...)
	return files
}

func (*Artifact) Id() string {
	return ""
}

func (a *Artifact) String() string {
	s := ""

	for k, v := range a.StateData {
		s += fmt.Sprintf("%s=%v ", k, v)
	}

	return s
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

func (a *Artifact) Destroy() error {
	a.StateData = nil
	return nil
}
