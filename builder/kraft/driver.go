package kraft

// Driver is the interface that has to be implemented to communicate with
// Kraft. The Driver interface also allows the steps to be tested since
// a mock driver can be shimmed in.
type Driver interface {
	Build(path, architecture, platform string) error

	Pull(source, workdir string) error

	Source(source string) error

	Update() error

	Set(options map[string]string) error
}
