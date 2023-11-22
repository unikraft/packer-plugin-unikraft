// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package unikraft

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/multiselect"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/target"
)

type Build struct {
	All          bool
	Architecture string
	DotConfig    string
	ForcePull    bool
	Jobs         int
	KernelDbg    bool
	Kraftfile    string
	NoCache      bool
	NoConfigure  bool
	NoFast       bool
	NoFetch      bool
	NoUpdate     bool
	Platform     string
	Rootfs       string
	SaveBuildLog string
	Target       string

	project app.Application
	workdir string
}

func (opts *Build) pull(ctx context.Context) error {
	var missingPacks []pack.Package
	auths := config.G[config.KraftKit](ctx).Auth

	if template := opts.project.Template(); template != nil {
		if stat, err := os.Stat(template.Path()); err != nil || !stat.IsDir() || opts.ForcePull {
			var templatePack pack.Package

			p, err := packmanager.G(ctx).Catalog(ctx,
				packmanager.WithName(template.Name()),
				packmanager.WithTypes(template.Type()),
				packmanager.WithVersion(template.Version()),
				packmanager.WithSource(template.Source()),
				packmanager.WithUpdate(opts.NoCache),
				packmanager.WithAuthConfig(auths),
			)
			if err != nil {
				return err
			}

			if len(p) == 0 {
				return fmt.Errorf("could not find: %s",
					unikraft.TypeNameVersion(template),
				)
			} else if len(p) > 1 {
				return fmt.Errorf("too many options for %s",
					unikraft.TypeNameVersion(template),
				)
			}

			templatePack = p[0]

			templatePack.Pull(
				ctx,
				pack.WithPullWorkdir(opts.workdir),
				// pack.WithPullChecksum(!opts.NoChecksum),
				pack.WithPullCache(!opts.NoCache),
				pack.WithPullAuthConfig(auths),
			)
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

	components, err := opts.project.Components(ctx)
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
			packmanager.WithUpdate(opts.NoCache),
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
			log.G(ctx).Warnf("too many options for %s, %d",
				unikraft.TypeNameVersion(component),
				len(p),
			)
			for _, p1 := range p {
				log.G(ctx).Warnf(" - %s", p1.String())
			}
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
			p.Pull(
				ctx,
				pack.WithPullWorkdir(opts.workdir),
				// pack.WithPullChecksum(!opts.NoChecksum),
				pack.WithPullCache(!opts.NoCache),
				pack.WithPullAuthConfig(auths),
			)
		}
	}

	return nil
}

func (opts *Build) BuildCmd(ctx context.Context, args ...string) error {
	var err error

	if len(args) == 0 {
		opts.workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		opts.workdir = args[0]
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil && errors.Is(err, app.ErrNoKraftfile) {
		return fmt.Errorf("cannot build project directory without a Kraftfile")
	} else if err != nil {
		return fmt.Errorf("could not initialize project directory: %w", err)
	}

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	selected := opts.project.Targets()
	if len(selected) == 0 {
		return fmt.Errorf("no targets to build")
	}
	if !opts.All {
		selected = target.Filter(
			selected,
			opts.Architecture,
			opts.Platform,
			opts.Target,
		)

		if !config.G[config.KraftKit](ctx).NoPrompt {
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

	if opts.ForcePull || !opts.NoUpdate {
		err := packmanager.G(ctx).Update(ctx)
		if err != nil {
			return err
		}
	}

	if err := opts.pull(ctx); err != nil {
		return err
	}

	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(!opts.NoFast && !config.G[config.KraftKit](ctx).NoParallel))
	}

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		if !opts.NoConfigure {
			err := opts.project.Configure(
				ctx,
				targ, // Target-specific options
				nil,  // No extra configuration options
				make.WithSilent(true),
				make.WithExecOptions(
					exec.WithStdin(iostreams.G(ctx).In),
					exec.WithStdout(log.G(ctx).Writer()),
					exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
				),
			)
			if err != nil {
				return err
			}
		}

		err := opts.project.Build(
			ctx,
			targ, // Target-specific options
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
			return err
		}
	}

	return nil
}

type Pkg struct {
	Architecture string
	Args         []string
	Dbg          bool
	Force        bool
	Format       string
	Kernel       string
	Kraftfile    string
	Name         string
	NoKConfig    bool
	Output       string
	Platform     string
	Project      app.Application
	Push         bool
	Rootfs       string
	Strategy     packmanager.MergeStrategy
	Target       string
	Workdir      string

	packopts []packmanager.PackOption
	pm       packmanager.PackageManager
}

// buildRootfs generates a rootfs based on the provided path
func (opts *Pkg) buildRootfs(ctx context.Context) error {
	if opts.Rootfs == "" {
		if opts.Project != nil && opts.Project.Rootfs() != "" {
			opts.Rootfs = opts.Project.Rootfs()
		} else {
			return nil
		}
	}

	ramfs, err := initrd.New(ctx, opts.Rootfs,
		initrd.WithOutput(filepath.Join(opts.Workdir, unikraft.BuildDir, initrd.DefaultInitramfsFileName)),
		initrd.WithCacheDir(filepath.Join(opts.Workdir, unikraft.VendorDir, "rootfs-cache")),
	)
	if err != nil {
		return fmt.Errorf("could not prepare initramfs: %w", err)
	}

	opts.Rootfs, err = ramfs.Build(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (opts *Pkg) initProject(ctx context.Context) error {
	var err error

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.Workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Interpret the project directory
	opts.Project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	return nil
}

func (opts *Pkg) PackCmd(ctx context.Context, args ...string) ([]pack.Package, error) {
	var err error

	if opts == nil {
		opts = &Pkg{}
	}

	opts.packopts = []packmanager.PackOption{}
	opts.Strategy = packmanager.StrategyOverwrite

	if len(args) == 0 {
		opts.Workdir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else if len(opts.Workdir) == 0 {
		opts.Workdir = args[0]
	}

	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return nil, err
		}
	}
	if opts.Project.Unikraft(ctx) == nil {
		return nil, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.Name == "" {
		return nil, fmt.Errorf("cannot package without setting --name")
	}

	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return nil, fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	if config.G[config.KraftKit](ctx).NoPrompt && opts.Strategy == packmanager.StrategyPrompt {
		return nil, fmt.Errorf("cannot mix --strategy=prompt when --no-prompt is enabled in settings")
	}

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	if len(opts.Format) > 0 {
		// Switch the package manager the desired format for this target
		opts.pm, err = packmanager.G(ctx).From(pack.PackageFormat(opts.Format))
		if err != nil {
			return nil, err
		}
	} else {
		opts.pm = packmanager.G(ctx)
	}

	exists, err := opts.pm.Catalog(ctx,
		packmanager.WithName(opts.Name),
	)
	if err == nil && len(exists) > 0 {
		if opts.Strategy == packmanager.StrategyPrompt {
			strategy, err := selection.Select[packmanager.MergeStrategy](
				fmt.Sprintf("package '%s' already exists: how would you like to proceed?", opts.Name),
				packmanager.MergeStrategies()...,
			)
			if err != nil {
				return nil, err
			}

			opts.Strategy = *strategy
		}

		switch opts.Strategy {
		case packmanager.StrategyExit:
			return nil, fmt.Errorf("package already exists and merge strategy set to exit on conflict")

		// Set the merge strategy as an option that is then passed to the
		// package manager.
		default:
			opts.packopts = append(opts.packopts,
				packmanager.PackMergeStrategy(opts.Strategy),
			)
		}
	} else {
		opts.packopts = append(opts.packopts,
			packmanager.PackMergeStrategy(packmanager.StrategyMerge),
		)
	}

	if err := opts.buildRootfs(ctx); err != nil {
		return nil, fmt.Errorf("could not build rootfs: %w", err)
	}

	selected := opts.Project.Targets()
	if len(opts.Target) > 0 || len(opts.Architecture) > 0 || len(opts.Platform) > 0 {
		selected = target.Filter(opts.Project.Targets(), opts.Architecture, opts.Platform, opts.Target)
	}

	if len(selected) > 1 && !config.G[config.KraftKit](ctx).NoPrompt {
		selected, err = multiselect.MultiSelect[target.Target]("select what to package", opts.Project.Targets()...)
		if err != nil {
			return nil, err
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("nothing selected to package")
	}

	i := 0

	var result []pack.Package
	var havePackages bool

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		baseopts := opts.packopts

		// If no arguments have been specified, use the ones which are default and
		// that have been included in the package.
		if len(opts.Args) == 0 {
			opts.Args = targ.Command()
		}

		cmdShellArgs, err := shellwords.Parse(strings.Join(opts.Args, " "))
		if err != nil {
			return nil, err
		}

		// When i > 0, we have already applied the merge strategy.  Now, for all
		// targets, we actually do wish to merge these because they are part of
		// the same execution lifecycle.
		if i > 0 {
			baseopts = []packmanager.PackOption{
				packmanager.PackMergeStrategy(packmanager.StrategyMerge),
			}
		}

		havePackages = true

		popts := append(baseopts,
			packmanager.PackArgs(cmdShellArgs...),
			packmanager.PackInitrd(opts.Rootfs),
			packmanager.PackKConfig(!opts.NoKConfig),
			packmanager.PackName(opts.Name),
			packmanager.PackOutput(opts.Output),
		)

		if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
			popts = append(popts,
				packmanager.PackWithKernelVersion(ukversion.Value),
			)
		}

		more, err := opts.pm.Pack(ctx, targ, popts...)
		if err != nil {
			return nil, err
		}

		result = append(result, more...)

		i++
	}

	if !havePackages {
		switch true {
		case len(opts.Target) > 0:
			return nil, fmt.Errorf("no matching targets found for: %s", opts.Target)
		case len(opts.Architecture) > 0 && len(opts.Platform) == 0:
			return nil, fmt.Errorf("no matching targets found for architecture: %s", opts.Architecture)
		case len(opts.Architecture) == 0 && len(opts.Platform) > 0:
			return nil, fmt.Errorf("no matching targets found for platform: %s", opts.Platform)
		}
	}

	if opts.Push {
		for _, p := range result {
			err := p.Push(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

type Clean struct {
	Architecture string
	Kraftfile    string
	Platform     string
	Target       string
	Proper       bool
}

func (opts *Clean) CleanCmd(ctx context.Context, args []string) error {
	var err error
	workdir := ""

	// Delete everything for now for backwards compatibility
	opts.Proper = true

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	// Filter project targets by any provided CLI options
	targets := target.Filter(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	t, err := target.Select(targets)
	if err != nil {
		return err
	}

	if opts.Proper {
		return project.Properclean(ctx, t)
	}

	return project.Clean(ctx, t)
}

type Pull struct {
	All          bool
	Architecture string
	ForceCache   bool
	Kraftfile    string
	Manager      string
	NoChecksum   bool
	NoDeps       bool
	Platform     string
	WithDeps     bool
	Workdir      string
	KConfig      []string
}

func (opts *Pull) PullCmd(ctx context.Context, args []string) error {
	var err error
	var project app.Application

	workdir := opts.Workdir
	if len(workdir) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if len(args) == 0 {
		args = []string{workdir}
	}

	pm := packmanager.G(ctx)

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Manager))
		if err != nil {
			return err
		}
	}

	type pmQuery struct {
		pm    packmanager.PackageManager
		query []packmanager.QueryOption
	}

	// If `--all` is not set and either `--plat` or `--arch` are not set,
	// use the host platform and architecture, as the user is likely trying
	// to pull for their system by using "sensible defaults".
	if !opts.All {
		if opts.Architecture == "" {
			opts.Architecture, err = arch.HostArchitecture()
			if err != nil {
				return fmt.Errorf("could not determine host architecture: %w", err)
			}
		}

		if opts.Platform == "" {
			platform, _, err := platform.Detect(ctx)
			if err != nil {
				return fmt.Errorf("could not detect host platform: %w", err)
			}
			opts.Platform = platform.String()
		}
	}

	var queries []pmQuery

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		workdir = args[0]
		popts := []app.ProjectOption{}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		project, err := app.NewProjectFromOptions(
			ctx,
			append(popts, app.WithProjectWorkdir(workdir))...,
		)
		if err != nil {
			return err
		}

		if _, err = project.Components(ctx); err != nil {
			// Pull the template from the package manager
			var packages []pack.Package

			packages, err = pm.Catalog(ctx,
				packmanager.WithName(project.Template().Name()),
				packmanager.WithTypes(unikraft.ComponentTypeApp),
				packmanager.WithVersion(project.Template().Version()),
				packmanager.WithUpdate(opts.ForceCache),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithArchitecture(opts.Architecture),
			)
			if err != nil {
				return err
			}

			if len(packages) == 0 {
				return fmt.Errorf("could not find: %s", unikraft.TypeNameVersion(project.Template()))
			} else if len(packages) > 1 {
				return fmt.Errorf("too many options for %s", unikraft.TypeNameVersion(project.Template()))
			}

			err := packages[0].Pull(
				ctx,
				pack.WithPullWorkdir(workdir),
				// pack.WithPullChecksum(!opts.NoChecksum),
				// pack.WithPullCache(!opts.NoCache),
			)
			if err != nil {
				return err
			}
		}

		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			ctx,
			append(popts, app.WithProjectWorkdir(templateWorkdir))...,
		)
		if err != nil {
			return err
		}

		project, err = templateProject.MergeTemplate(ctx, project)
		if err != nil {
			return err
		}

		// List the components
		components, err := project.Components(ctx)
		if err != nil {
			return err
		}
		for _, c := range components {
			queries = append(queries, pmQuery{
				pm: pm,
				query: []packmanager.QueryOption{
					packmanager.WithName(c.Name()),
					packmanager.WithVersion(c.Version()),
					packmanager.WithSource(c.Source()),
					packmanager.WithTypes(c.Type()),
					packmanager.WithUpdate(!opts.ForceCache),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithArchitecture(opts.Architecture),
				},
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else if len(args) > 0 {
		for _, arg := range args {
			pm, compatible, err := pm.IsCompatible(ctx, arg,
				packmanager.WithUpdate(!opts.ForceCache),
			)
			if err != nil || !compatible {
				continue
			}

			queries = append(queries, pmQuery{
				pm: pm,
				query: []packmanager.QueryOption{
					packmanager.WithUpdate(!opts.ForceCache),
					packmanager.WithName(arg),
					packmanager.WithArchitecture(opts.Architecture),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithKConfig(opts.KConfig),
				},
			})
		}
	}

	for _, c := range queries {
		query := packmanager.NewQuery(c.query...)
		next, err := c.pm.Catalog(ctx, c.query...)
		if err != nil {
			log.G(ctx).
				WithField("format", pm.Format().String()).
				WithField("name", query.Name()).
				Warn(err)
			continue
		}

		if len(next) == 0 {
			log.G(ctx).Warnf("could not find %s", query.String())
			continue
		}

		for _, p := range next {
			p := p
			err := p.Pull(
				ctx,
				pack.WithPullWorkdir(workdir),
				pack.WithPullChecksum(!opts.NoChecksum),
				pack.WithPullCache(opts.ForceCache),
			)
			if err != nil {
				return err
			}
		}
	}

	if project != nil {
		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo(ctx))
	}

	return nil
}

type Source struct {
	Force bool
}

func (opts *Source) SourceCmd(ctx context.Context, args []string) error {
	for _, source := range args {
		if !opts.Force {
			_, compatible, err := packmanager.G(ctx).IsCompatible(ctx,
				source,
				packmanager.WithUpdate(true),
			)
			if err != nil {
				return err
			} else if !compatible {
				return errors.New("incompatible package manager")
			}

			err = packmanager.G(ctx).AddSource(ctx, source)
			if err != nil {
				return err
			}
		}

		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if source == manifest {
				log.G(ctx).Warnf("manifest already saved: %s", source)
				return nil
			}
		}

		config.G[config.KraftKit](ctx).Unikraft.Manifests = append(
			config.G[config.KraftKit](ctx).Unikraft.Manifests,
			source,
		)

		if err := config.M[config.KraftKit](ctx).Write(true); err != nil {
			return err
		}
	}

	return nil
}

type Unsource struct{}

func (opts *Unsource) UnsourceCmd(ctx context.Context, args []string) error {
	for _, source := range args {
		manifests := []string{}

		var manifestRemoved bool
		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if source != manifest {
				manifests = append(manifests, manifest)
			} else {
				manifestRemoved = true
			}
		}

		if !manifestRemoved {
			log.G(ctx).Warnf("manifest not found: %s", source)
			return nil
		}

		config.G[config.KraftKit](ctx).Unikraft.Manifests = manifests

		if err := config.M[config.KraftKit](ctx).Write(false); err != nil {
			return err
		}
	}

	return nil
}

type Update struct {
	Manager string
}

func (opts *Update) UpdateCmd(ctx context.Context, args []string) error {
	var err error

	pm := packmanager.G(ctx)

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "all" {
		pm, err = pm.From(pack.PackageFormat(opts.Manager))
		if err != nil {
			return err
		}

		err := pm.Update(ctx)
		if err != nil {
			return err
		}
	} else {
		umbrella, err := packmanager.PackageManagers()
		if err != nil {
			return err
		}

		for _, pm := range umbrella {
			err := pm.Update(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type Set struct {
	Kraftfile string
	Workdir   string
}

func (opts *Set) SetCmd(ctx context.Context, args []string) error {
	var err error

	workdir := ""
	confOpts := []string{}

	// Skip if nothing can be set
	if len(args) == 0 {
		return fmt.Errorf("no options to set")
	}

	// Set the working directory (remove the argument if it exists)
	if opts.Workdir != "" {
		workdir = opts.Workdir
	} else {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	// Set the configuration options, skip the first one if needed
	for _, arg := range args {
		if !strings.ContainsRune(arg, '=') || strings.HasSuffix(arg, "=") {
			return fmt.Errorf("invalid or malformed argument: %s", arg)
		}

		confOpts = append(confOpts, arg)
	}

	// Check if dotconfig exists in workdir
	dotconfig := fmt.Sprintf("%s/.config", workdir)

	// Check if the file exists
	// TODO: offer option to start in interactive mode
	if _, err := os.Stat(dotconfig); os.IsNotExist(err) {
		return fmt.Errorf("dotconfig file does not exist: %s", dotconfig)
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
		app.WithProjectConfig(confOpts),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	return project.Set(ctx, nil)
}
