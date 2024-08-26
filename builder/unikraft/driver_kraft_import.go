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

	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
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
	Env          []string
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
	Target       target.Target
	TargetName   string

	project    app.Application
	workdir    string
	statistics map[string]string
}

func (opts *Build) initProject(ctx context.Context) error {
	var err error

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Interpret the project directory
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	return nil
}

func (opts *Build) BuildCmd(ctx context.Context, args ...string) error {
	var err error

	if opts == nil {
		opts = &Build{}
	}

	if len(opts.workdir) == 0 {
		if len(args) == 0 {
			opts.workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			opts.workdir = args[0]
		}
	}

	opts.statistics = map[string]string{}

	var build builder
	builders := builders()

	// Iterate through the list of built-in builders which sequentially tests
	// the current context and Kraftfile match specific requirements towards
	// performing a type of build.
	for _, candidate := range builders {
		log.G(ctx).
			WithField("builder", candidate.String()).
			Trace("checking buildability")

		capable, err := candidate.Buildable(ctx, opts, args...)
		if capable && err == nil {
			build = candidate
			break
		}
	}

	if build == nil {
		return fmt.Errorf("could not determine what or how to build from the given context")
	}

	log.G(ctx).WithField("builder", build.String()).Debug("using")

	if err := build.Prepare(ctx, opts, args...); err != nil {
		return fmt.Errorf("could not complete build: %w", err)
	}

	if opts.Rootfs, _, _, err = BuildRootfs(ctx, opts.workdir, opts.Rootfs, false, opts.Target.Architecture().String()); err != nil {
		return err
	}

	// Set the root file system for the project, since typically a packaging step
	// may occur after a build, and the root file system is required for packaging
	// and the packaging step may perform a build of the rootfs again.  Ultimately
	// this prevents re-builds.
	opts.project.SetRootfs(opts.Rootfs)

	err = build.Build(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not complete build: %w", err)
	}

	// NOTE(craciunoiuc): This is currently a workaround to remove empty
	// Makefile.uk files generated wrongly by the build system. Until this
	// is fixed we just delete.
	//
	// See: https://github.com/unikraft/unikraft/issues/1456
	make := filepath.Join(opts.workdir, "Makefile.uk")
	if finfo, err := os.Stat(make); err == nil && finfo.Size() == 0 {
		err := os.Remove(make)
		if err != nil {
			return fmt.Errorf("removing empty Makefile.uk: %w", err)
		}
	}

	return nil
}

type Pkg struct {
	Architecture string
	Args         []string
	Dbg          bool
	Env          []string
	Force        bool
	Format       string
	Kernel       string
	Kraftfile    string
	Labels       []string
	Name         string
	NoKConfig    bool
	NoPull       bool
	Output       string
	Platform     string
	Project      app.Application
	Push         bool
	Rootfs       string
	Runtime      string
	Strategy     packmanager.MergeStrategy
	Target       string
	Workdir      string

	packopts []packmanager.PackOption
	pm       packmanager.PackageManager
}

func (opts *Pkg) aggregateEnvs() []string {
	envs := make(map[string]string)

	if opts.Project != nil && opts.Project.Env() != nil {
		envs = opts.Project.Env()
	}

	// Add the cli environment
	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			parts := strings.SplitN(env, "=", 2)
			envs[parts[0]] = parts[1]
			continue
		}

		envs[env] = os.Getenv(env)
	}

	// Aggregate all the environment variables
	var env []string
	for k, v := range envs {
		env = append(env, k+"="+v)
	}

	return env
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

	if opts.Workdir == "" {
		if len(args) == 0 {
			opts.Workdir, err = os.Getwd()
			if err != nil {
				return nil, err
			}
		}
	}

	if len(args) != 0 {
		opts.Workdir = args[0]
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

	var exists []pack.Package

	exists, err = opts.pm.Catalog(ctx,
		packmanager.WithName(opts.Name),
	)
	if err != nil {
		return nil, fmt.Errorf("could not start the process tree: %w", err)
	}

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
		case packmanager.StrategyAbort:
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

	var pkgr packager

	packagers := packagers()

	// Iterate through the list of built-in builders which sequentially tests
	// the current context and Kraftfile match specific requirements towards
	// performing a type of build.
	for _, candidate := range packagers {
		log.G(ctx).
			WithField("packager", candidate.String()).
			Trace("checking compatibility")

		capable, err := candidate.Packagable(ctx, opts, args...)
		if capable && err == nil {
			pkgr = candidate
			break
		}

		log.G(ctx).
			WithError(err).
			WithField("packager", candidate.String()).
			Trace("incompatbile")
	}

	if pkgr == nil {
		return nil, fmt.Errorf("could not determine what or how to package from the given context")
	}

	log.G(ctx).WithField("packager", pkgr.String()).Debug("using")

	packs, err := pkgr.Pack(ctx, opts, args...)
	if err != nil {
		return nil, fmt.Errorf("could not package: %w", err)
	}

	if opts.Push {

		for _, p := range packs {
			p := p

			err := p.Push(ctx)
			if err != nil {
				return packs, err
			}
		}
	}

	return packs, nil
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

	if opts.Proper && len(targets) > 0 {
		return project.Properclean(ctx, targets[0])
	}

	t, err := target.Select(targets)
	if err != nil {
		return err
	}

	return project.Clean(ctx, t)
}

type Pull struct {
	All          bool
	Architecture string
	ForceCache   bool
	Format       string
	Kraftfile    string
	Manager      string
	NoChecksum   bool
	NoDeps       bool
	Output       string
	Platform     string
	WithDeps     bool
	Workdir      string
	KConfig      []string

	update bool
}

func (opts *Pull) PullCmd(ctx context.Context, args []string) error {
	var err error
	var project app.Application

	opts.update = true

	if len(opts.Workdir) == 0 {
		opts.Workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if len(opts.Output) == 0 {
		opts.Output = opts.Workdir
	}

	if len(args) == 0 {
		args = []string{opts.Workdir}
	}

	pm := packmanager.G(ctx)

	// Force a particular package manager
	if len(opts.Format) > 0 && opts.Format != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Format))
		if err != nil {
			return err
		}
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

	var queries [][]packmanager.QueryOption

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		log.G(ctx).Debug("ignoring -w|--workdir")
		opts.Workdir = args[0]
		popts := []app.ProjectOption{}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		project, err := app.NewProjectFromOptions(
			ctx,
			append(popts, app.WithProjectWorkdir(opts.Workdir))...,
		)
		if err != nil {
			return err
		}

		if _, err = project.Components(ctx); err != nil {
			var pullPack pack.Package
			var packages []pack.Package

			// Pull the template from the package manager
			if project.Template() != nil {
				qopts := []packmanager.QueryOption{
					packmanager.WithName(project.Template().Name()),
					packmanager.WithTypes(unikraft.ComponentTypeApp),
					packmanager.WithVersion(project.Template().Version()),
					packmanager.WithRemote(opts.update),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithArchitecture(opts.Architecture),
					packmanager.WithLocal(true),
				}
				packages, err = pm.Catalog(ctx, qopts...)
				if err != nil {
					return err
				}

				if len(packages) == 0 {
					return fmt.Errorf("could not find: %s based on %s", unikraft.TypeNameVersion(project.Template()), packmanager.NewQuery(qopts...).String())
				}

				if len(packages) == 1 {
					pullPack = packages[0]
				} else if len(packages) > 1 {
					if config.G[config.KraftKit](ctx).NoPrompt {
						for _, p := range packages {
							log.G(ctx).
								WithField("template", p.String()).
								Warn("possible")
						}

						return fmt.Errorf("too many options for %s and prompting has been disabled",
							project.Template().String(),
						)
					}

					selected, err := selection.Select[pack.Package]("select possible template", packages...)
					if err != nil {
						return err
					}

					pullPack = *selected
				}

				return pullPack.Pull(
					ctx,
					pack.WithPullWorkdir(opts.Output),
				)
			}

			templateWorkdir, err := unikraft.PlaceComponent(opts.Output, project.Template().Type(), project.Template().Name())
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
		}

		// List the components
		components, err := project.Components(ctx)
		if err != nil {
			return err
		}
		for _, c := range components {
			queries = append(queries, []packmanager.QueryOption{
				packmanager.WithName(c.Name()),
				packmanager.WithVersion(c.Version()),
				packmanager.WithSource(c.Source()),
				packmanager.WithTypes(c.Type()),
				packmanager.WithRemote(opts.update),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithArchitecture(opts.Architecture),
			})
		}

		if project.Runtime() != nil {
			queries = append(queries, []packmanager.QueryOption{
				packmanager.WithName(project.Runtime().Name()),
				packmanager.WithVersion(project.Runtime().Version()),
				packmanager.WithRemote(opts.update),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithArchitecture(opts.Architecture),
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else if len(args) > 0 {
		for _, arg := range args {
			queries = append(queries, []packmanager.QueryOption{
				packmanager.WithRemote(opts.update),
				packmanager.WithName(arg),
				packmanager.WithArchitecture(opts.Architecture),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithKConfig(opts.KConfig),
			})
		}
	}

	if len(queries) == 0 {
		return fmt.Errorf("no components to pull")
	}

	var found []pack.Package
	var foundErr bool

	for _, qopts := range queries {
		qopts := qopts
		query := packmanager.NewQuery(qopts...)
		more, err := pm.Catalog(ctx, qopts...)
		if err != nil {
			log.G(ctx).
				WithField("format", pm.Format().String()).
				WithField("name", query.Name()).
				Warn(err)
			foundErr = true
			return nil
		}

		if len(more) == 0 {
			opts.update = true
			foundErr = true
			continue
		}

		found = append(found, more...)
	}

	// Try again with a remote search
	if foundErr && (len(found) == 0 || !opts.update) {
		for _, qopts := range queries {
			qopts := qopts
			query := packmanager.NewQuery(qopts...)
			more, err := pm.Catalog(ctx, append(
				qopts,
				packmanager.WithRemote(true),
			)...)
			if err != nil {
				log.G(ctx).
					WithField("format", pm.Format().String()).
					WithField("name", query.Name()).
					Warn(err)
				return nil
			}

			if len(more) == 0 {
				return fmt.Errorf("could not find %s", query.String())
			}

			found = append(found, more...)
		}
	}

	for _, p := range found {
		p := p
		err := p.Pull(
			ctx,
			pack.WithPullWorkdir(opts.Output),
			pack.WithPullChecksum(!opts.NoChecksum),
			pack.WithPullCache(!opts.update),
		)

		if err != nil {
			return err
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
				packmanager.WithRemote(true),
			)
			if err != nil {
				return err
			} else if !compatible {
				return errors.New("incompatible package manager")
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

		return pm.Update(ctx)
	} else {
		umbrella, err := packmanager.PackageManagers()
		if err != nil {
			return err
		}
		for _, pm := range umbrella {
			pm := pm // Go closures
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

	log.G(ctx).Warnf("This command is DEPRECATED and should not be used")

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
