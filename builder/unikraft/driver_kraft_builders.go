// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package unikraft

import (
	"context"
	"fmt"
	"os"
	plainexec "os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/export/v0/posixenviron"
	"kraftkit.sh/unikraft/target"
)

func rewrapAsKraftCloudPackage(name string) string {
	name = strings.Replace(name, "unikarft.org/", "index.unikraft.io/", 1)

	if strings.HasPrefix(name, "unikraft.io") {
		name = "index." + name
	} else if strings.Contains(name, "/") && !strings.Contains(name, "unikraft.io") {
		name = "index.unikraft.io/" + name
	} else if !strings.HasPrefix(name, "index.unikraft.io") {
		name = "index.unikraft.io/official/" + name
	}

	return name
}

func BuildRootfs(ctx context.Context, workdir, rootfs string, compress bool, arch string) (string, []string, []string, error) {
	if rootfs == "" {
		return "", nil, nil, nil
	}

	var cmds []string
	var envs []string

	ramfs, err := initrd.New(ctx, rootfs,
		initrd.WithWorkdir(workdir),
		initrd.WithOutput(filepath.Join(
			workdir,
			unikraft.BuildDir,
			fmt.Sprintf(initrd.DefaultInitramfsArchFileName, arch),
		)),
		initrd.WithCacheDir(filepath.Join(
			workdir,
			unikraft.VendorDir,
			"rootfs-cache",
		)),
		initrd.WithArchitecture(arch),
		initrd.WithCompression(compress),
	)
	if err != nil {
		return "", nil, nil, fmt.Errorf("could not initialize initramfs builder: %w", err)
	}

	rootfs, err = ramfs.Build(ctx)
	if err != nil {
		return "", nil, nil, err
	}

	// Always overwrite the existing cmds and envs, considering this will
	// be the same regardless of the target.
	cmds = ramfs.Args()
	envs = ramfs.Env()

	return rootfs, cmds, envs, nil
}

type builder interface {
	fmt.Stringer

	Buildable(context.Context, *Build, ...string) (bool, error)

	Prepare(context.Context, *Build, ...string) error

	Build(context.Context, *Build, ...string) error

	Statistics(context.Context, *Build, ...string) error
}

func builders() []builder {
	return []builder{
		&builderKraftfileUnikraft{},
		&builderKraftfileRuntime{},
		&builderDockerfile{},
	}
}

type builderKraftfileUnikraft struct {
	nameWidth int
}

// String implements fmt.Stringer.
func (build *builderKraftfileUnikraft) String() string {
	return "kraftfile-unikraft"
}

// Buildable implements builder.
func (build *builderKraftfileUnikraft) Buildable(ctx context.Context, opts *Build, args ...string) (bool, error) {
	if opts.project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.project.Unikraft(ctx) == nil && opts.project.Template() == nil {
		return false, fmt.Errorf("cannot build without unikraft core specification")
	}

	if opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	return true, nil
}

// Calculate lines of code in a kernel image.
// Requires objdump to be installed and debug symbols to be enabled.
func linesOfCode(ctx context.Context, opts *Build) (int64, error) {
	objdumpPath, err := plainexec.LookPath("objdump")
	if err != nil {
		log.G(ctx).Warn("objdump not found, skipping LoC statistics")
		return 0, nil
	}
	cmd := plainexec.CommandContext(ctx, objdumpPath, "-dl", opts.Target.KernelDbg())
	cmd.Stderr = log.G(ctx).WriterLevel(logrus.DebugLevel)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("running objdump: %w", err)
	}

	uniqueLines := map[string]bool{}
	filterRegex1 := regexp.MustCompile(`^/.*$`)
	filterRegex2 := regexp.MustCompile(`^/[/*].*$`)
	filterRegex3 := regexp.MustCompile(` [(]discriminator [0-9]+[)]`)
	for _, line := range strings.Split(string(out), "\n") {
		if filterRegex1.FindString(line) != "" &&
			filterRegex2.FindString(line) == "" {
			uniqueLines[filterRegex3.ReplaceAllString(line, "")] = true
		}
	}

	return int64(len(uniqueLines)), nil
}

func (build *builderKraftfileUnikraft) pull(ctx context.Context, opts *Build, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	auths := config.G[config.KraftKit](ctx).Auth

	if template := opts.project.Template(); template != nil {
		if stat, err := os.Stat(template.Path()); err != nil || !stat.IsDir() || opts.ForcePull {
			var templatePack pack.Package
			var packs []pack.Package

			p, err := packmanager.G(ctx).Catalog(ctx,
				packmanager.WithName(template.Name()),
				packmanager.WithTypes(template.Type()),
				packmanager.WithVersion(template.Version()),
				packmanager.WithSource(template.Source()),
				packmanager.WithRemote(opts.NoCache),
				packmanager.WithAuthConfig(auths),
			)
			if err != nil {
				return err
			}

			if len(p) == 0 {
				return fmt.Errorf("could not find: %s",
					unikraft.TypeNameVersion(template),
				)
			}

			packs = append(packs, p...)

			if len(packs) == 1 {
				templatePack = packs[0]
			} else if len(packs) > 1 {
				if config.G[config.KraftKit](ctx).NoPrompt {
					for _, p := range packs {
						log.G(ctx).
							WithField("template", p.String()).
							Warn("possible")
					}

					return fmt.Errorf("too many options for %s and prompting has been disabled",
						unikraft.TypeNameVersion(template),
					)
				}

				selected, err := selection.Select[pack.Package]("select possible template", packs...)
				if err != nil {
					return err
				}

				templatePack = *selected
			}

			templatePack.Pull(
				ctx,
				pack.WithPullWorkdir(opts.workdir),
				// pack.WithPullChecksum(!opts.NoChecksum),
				pack.WithPullCache(!opts.NoCache),
				pack.WithPullAuthConfig(auths),
			)
			if err != nil {
				return err
			}
		}

		templateProject, err := app.NewProjectFromOptions(ctx,
			app.WithProjectWorkdir(template.Path()),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		// Overwrite template with user options
		opts.project, err = opts.project.MergeTemplate(ctx, templateProject)
		if err != nil {
			return err
		}
	}

	components, err := opts.project.Components(ctx, opts.Target)
	if err != nil {
		return err
	}
	for _, component := range components {
		// Skip "finding" the component if path is the same as the source (which
		// means that the source code is already available as it is a directory on
		// disk.  In this scenario, the developer is likely hacking the particular
		// microlibrary/component.
		if component.Path() == component.Source() {
			continue
		}

		// Only continue to find and pull the component if it does not exist
		// locally or the user has requested to --force-pull.
		if stat, err := os.Stat(component.Path()); err == nil && stat.IsDir() && !opts.ForcePull {
			continue
		}

		component := component // loop closure
		auths := auths

		if f, err := os.Stat(component.Source()); err == nil && f.IsDir() {
			continue
		}

		p, err := packmanager.G(ctx).Catalog(ctx,
			packmanager.WithName(component.Name()),
			packmanager.WithTypes(component.Type()),
			packmanager.WithVersion(component.Version()),
			packmanager.WithSource(component.Source()),
			packmanager.WithRemote(opts.NoCache),
			packmanager.WithAuthConfig(auths),
		)
		if err != nil {
			return err
		}

		if len(p) == 0 {
			return fmt.Errorf("could not find: %s",
				unikraft.TypeNameVersion(component),
			)
		} else if len(p) > 1 {
			return fmt.Errorf("too many options for %s",
				unikraft.TypeNameVersion(component),
			)
		}

		missingPacks = append(missingPacks, p...)
	}

	if len(missingPacks) > 0 {
		for _, p := range missingPacks {
			p := p // loop closure
			auths := auths
			err := p.Pull(
				ctx,
				pack.WithPullWorkdir(opts.workdir),
				// pack.WithPullChecksum(!opts.NoChecksum),
				pack.WithPullCache(!opts.NoCache),
				pack.WithPullAuthConfig(auths),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (build *builderKraftfileUnikraft) Prepare(ctx context.Context, opts *Build, args ...string) error {
	build.nameWidth = -1
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	if opts.Target == nil {
		// Filter project targets by any provided CLI options
		selected := opts.project.Targets()
		if len(selected) == 0 {
			return fmt.Errorf("no targets to build")
		}
		if !opts.All {
			selected = target.Filter(
				selected,
				opts.Architecture,
				opts.Platform,
				opts.TargetName,
			)

			if !config.G[config.KraftKit](ctx).NoPrompt && len(selected) > 1 {
				res, err := target.Select(selected)
				if err != nil {
					return err
				}
				selected = []target.Target{res}
			}
		}

		if len(selected) == 0 {
			return fmt.Errorf("no targets selected to build")
		}

		opts.Target = selected[0]
	}

	// Calculate the width of the longest process name so that we can align the
	// two independent processtrees if we are using "render" mode (aka the fancy
	// mode is enabled).
	if !norender {
		// The longest word is "configuring" (which is 11 characters long), plus
		// additional space characters (2 characters), brackets (2 characters) the
		// name of the project and the target/plat string (which is variable in
		// length).
		if newLen := len((opts.Target).Name()) + len(target.TargetPlatArchName(opts.Target)) + 15; newLen > build.nameWidth {
			build.nameWidth = newLen
		}

		components, err := opts.project.Components(ctx, opts.Target)
		if err != nil {
			return fmt.Errorf("could not get list of components: %w", err)
		}

		// The longest word is "pulling" (which is 7 characters long),plus
		// additional space characters (1 character).
		for _, component := range components {
			if newLen := len(unikraft.TypeNameVersion(component)) + 8; newLen > build.nameWidth {
				build.nameWidth = newLen
			}
		}
	}

	if opts.ForcePull || !opts.NoUpdate {
		if err := packmanager.G(ctx).Update(ctx); err != nil {
			return err
		}
	}

	return build.pull(ctx, opts, norender, build.nameWidth)
}

func (build *builderKraftfileUnikraft) Build(ctx context.Context, opts *Build, args ...string) error {
	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(!opts.NoFast && !config.G[config.KraftKit](ctx).NoParallel))
	}

	allEnvs := map[string]string{}
	for k, v := range opts.project.Env() {
		allEnvs[k] = v

		if v == "" {
			allEnvs[k] = os.Getenv(k)
		}
	}

	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			parts := strings.SplitN(env, "=", 2)
			allEnvs[parts[0]] = parts[1]
		} else {
			allEnvs[env] = os.Getenv(env)
		}
	}

	// There might already be environment variables in the project Kconfig,
	// so we need to be careful with indexing
	counter := 1
	envKconfig := kconfig.KeyValueMap{}
	for k, v := range allEnvs {
		for counter <= posixenviron.DefaultCompiledInLimit {
			val, found := opts.project.KConfig().Get(fmt.Sprintf("LIBPOSIX_ENVIRON_ENVP%d", counter))
			if !found || val.Value == "" {
				break
			}
			counter += 1
		}

		if counter > posixenviron.DefaultCompiledInLimit {
			log.G(ctx).Warnf("cannot compile in more than %d environment variables, skipping %s", posixenviron.DefaultCompiledInLimit, k)
			continue
		}

		envKconfig.Set(fmt.Sprintf("CONFIG_LIBPOSIX_ENVIRON_ENVP%d", counter), fmt.Sprintf("%s=%s", k, v))
		counter++
	}

	err := opts.project.Configure(
		ctx,
		opts.Target, // Target-specific options
		envKconfig,  // Extra Kconfigs for compiled in environment variables
		make.WithSilent(true),
		make.WithExecOptions(
			exec.WithStdin(iostreams.G(ctx).In),
			exec.WithStdout(log.G(ctx).Writer()),
			exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
		),
	)
	if err != nil {
		return fmt.Errorf("configure failed: %w", err)
	}

	err = opts.project.Build(
		ctx,
		opts.Target, // Target-specific options
		app.WithBuildMakeOptions(append(mopts,
			make.WithExecOptions(
				exec.WithStdout(log.G(ctx).Writer()),
				exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
				// exec.WithOSEnv(true),
			),
		)...),
		app.WithBuildLogFile(opts.SaveBuildLog),
	)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	return nil
}

func (build *builderKraftfileUnikraft) Statistics(ctx context.Context, opts *Build, args ...string) error {
	lines, err := linesOfCode(ctx, opts)
	if lines > 1 {
		opts.statistics["lines of code"] = fmt.Sprintf("%d", lines)
	}
	return err
}

type builderKraftfileRuntime struct{}

// String implements fmt.Stringer.
func (build *builderKraftfileRuntime) String() string {
	return "kraftfile-runtime"
}

// Buildable implements builder.
func (build *builderKraftfileRuntime) Buildable(ctx context.Context, opts *Build, args ...string) (bool, error) {
	if opts.project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.project.Runtime() == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	return true, nil
}

func (*builderKraftfileRuntime) Prepare(ctx context.Context, opts *Build, _ ...string) error {
	var (
		selected *pack.Package
		packs    []pack.Package
		kconfigs []string
		err      error
	)

	name := opts.project.Runtime().Name()
	if opts.Platform == "kraftcloud" || (opts.project.Runtime().Platform() != nil && opts.project.Runtime().Platform().Name() == "kraftcloud") {
		name = rewrapAsKraftCloudPackage(name)
	}

	qopts := []packmanager.QueryOption{
		packmanager.WithName(name),
		packmanager.WithVersion(opts.project.Runtime().Version()),
	}

	qopts = append(qopts,
		packmanager.WithArchitecture(opts.Architecture),
		packmanager.WithPlatform(opts.Platform),
		packmanager.WithKConfig(kconfigs),
	)

	packs, err = packmanager.G(ctx).Catalog(ctx, append(qopts, packmanager.WithRemote(false))...)
	if err != nil {
		return fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		// Try again with a remote update request.  Save this to qopts in case we
		// need to call `Catalog` again.
		packs, err = packmanager.G(ctx).Catalog(ctx, append(qopts, packmanager.WithRemote(true))...)
		if err != nil {
			return fmt.Errorf("could not query catalog: %w", err)
		}
	}
	if err != nil {
		return err
	}

	if len(packs) == 0 {
		if len(opts.Platform) > 0 && len(opts.Architecture) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s:%s' (%s/%s)",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
				opts.Platform,
				opts.Architecture,
			)
		} else if len(opts.Architecture) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' architecture",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
				opts.Architecture,
			)
		} else if len(opts.Platform) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' platform",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
				opts.Platform,
			)
		} else {
			return fmt.Errorf(
				"could not find runtime %s:%s",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
			)
		}
	} else if len(packs) == 1 {
		selected = &packs[0]
	} else if len(packs) > 1 {
		// If a target has been previously selected, we can use this to filter the
		// returned list of packages based on its platform and architecture.

		selected, err = selection.Select("multiple runtimes available", packs...)
		if err != nil {
			return err
		}
	}

	targ := (*selected).(target.Target)
	opts.Target = targ

	return nil
}

func (*builderKraftfileRuntime) Build(_ context.Context, _ *Build, _ ...string) error {
	return nil
}

func (*builderKraftfileRuntime) Statistics(ctx context.Context, opts *Build, args ...string) error {
	return fmt.Errorf("cannot calculate statistics of pre-built unikernel runtime")
}

type builderDockerfile struct{}

// String implements fmt.Stringer.
func (build *builderDockerfile) String() string {
	return "dockerfile"
}

// Buildable implements builder.
func (build *builderDockerfile) Buildable(ctx context.Context, opts *Build, args ...string) (bool, error) {
	if opts.project == nil {
		// Do not capture the the project is not initialized, as we can still build
		// the unikernel using the Dockerfile provided with the `--rootfs`.
		_ = opts.initProject(ctx)
	}

	if opts.project != nil && opts.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	// TODO(nderjung): This is a very naiive check and should be improved,
	// potentially using an external library which parses the Dockerfile syntax.
	// In most cases, however, the Dockerfile is usually named `Dockerfile`.
	if !strings.Contains(strings.ToLower(opts.Rootfs), "dockerfile") {
		return false, fmt.Errorf("file is not a Dockerfile")
	}

	return true, nil
}

// Prepare implements builder.
func (*builderDockerfile) Prepare(ctx context.Context, opts *Build, _ ...string) (err error) {
	return (&builderKraftfileRuntime{}).Prepare(ctx, opts)
}

// Build implements builder.
func (*builderDockerfile) Build(_ context.Context, _ *Build, _ ...string) error {
	return nil
}

// Statistics implements builder.
func (*builderDockerfile) Statistics(ctx context.Context, opts *Build, args ...string) error {
	return fmt.Errorf("cannot calculate statistics of pre-built unikernel runtime")
}
