package unikraft

type MockDriver struct {
	BuildCalled       bool
	BuildPath         string
	BuildArchitecture string
	BuildPlatform     string
	BuildTarget       string

	PkgCalled       bool
	PkgArchitecture string
	PkgPlatform     string
	PkgTarget       string
	PkgPush         bool

	CleanCalled bool
	CleanPath   string

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

func (d *MockDriver) Build(path, architecture, platform, target string) error {
	d.BuildCalled = true
	d.BuildPath = path
	d.BuildArchitecture = architecture
	d.BuildPlatform = platform
	d.BuildTarget = target
	return nil
}

func (d *MockDriver) Pkg(architecture, platform, target, pkgName string, push bool) error {
	d.PkgArchitecture = architecture
	d.PkgPlatform = platform
	d.PkgTarget = target
	d.PkgCalled = true
	d.PkgPush = push
	return nil
}

func (d *MockDriver) Clean(path string) error {
	d.CleanCalled = true
	d.CleanPath = path
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
