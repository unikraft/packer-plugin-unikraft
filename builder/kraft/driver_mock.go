package kraft

type MockDriver struct {
	BuildCalled       bool
	BuildPath         string
	BuildArchitecture string
	BuildPlatform     string

	ProperCleanCalled bool
	ProperCleanPath   string

	PullCalled  bool
	PullSource  string
	PullWorkdir string

	SourceCalled bool
	SourceSource string

	UnsourceCalled bool
	UnsourceSource string

	UpdateCalled bool

	SetCalled  bool
	SetOptions map[string]string

	UnsetCalled  bool
	UnsetOptions []string
}

func (d *MockDriver) Build(path, architecture, platform string) error {
	d.BuildCalled = true
	d.BuildPath = path
	d.BuildArchitecture = architecture
	d.BuildPlatform = platform
	return nil
}

func (d *MockDriver) ProperClean(path string) error {
	d.ProperCleanCalled = true
	d.ProperCleanPath = path
	return nil
}

func (d *MockDriver) Pull(source, workdir string) error {
	d.PullCalled = true
	d.PullSource = source
	d.PullWorkdir = workdir
	return nil
}

func (d *MockDriver) Source(source string) error {
	d.SourceCalled = true
	d.SourceSource = source
	return nil
}

func (d *MockDriver) Unsource(source string) error {
	d.UnsourceCalled = true
	d.UnsourceSource = source
	return nil
}

func (d *MockDriver) Update() error {
	d.UpdateCalled = true
	return nil
}

func (d *MockDriver) Set(options map[string]string) error {
	d.SetCalled = true
	d.SetOptions = options
	return nil
}

func (d *MockDriver) Unset(options []string) error {
	d.UnsetCalled = true
	d.UnsetOptions = options
	return nil
}
