package unikraft

// Driver is the interface that has to be implemented to communicate with
// Kraft. The Driver interface also allows the steps to be tested since
// a mock driver can be shimmed in.
type Driver interface {
	Build(path, architecture, platform, target string) error

	Pkg(architecture, platform, target, pkgName, rootfs, workdir string, push bool) error

	Clean(path string) error

	Pull(source, workdir string) error

	Set(options map[string]string) error

	Source(source string) error

	Unsource(source string) error

	Update() error
}
