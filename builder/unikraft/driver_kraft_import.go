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
	"strings"

	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type Build struct {
	Architecture string
	DotConfig    string
	Fast         bool
	Jobs         int
	KernelDbg    bool
	NoCache      bool
	NoConfigure  bool
	NoFetch      bool
	NoPrepare    bool
	NoPull       bool
	Platform     string
	SaveBuildLog string
	Target       string
}

func FilterTargets(targets target.Targets, arch, plat, targ string) target.Targets {
	var selected target.Targets

	type condition func(target.Target, string, string, string) bool

	conditions := []condition{
		// If no arguments are supplied
		func(t target.Target, arch, plat, targ string) bool {
			return len(targ) == 0 && len(arch) == 0 && len(plat) == 0
		},

		// If the `targ` is supplied and the target name match
		func(t target.Target, arch, plat, targ string) bool {
			return len(targ) > 0 && t.Name() == targ
		},

		// If only `arch` is supplied and the target's arch matches
		func(t target.Target, arch, plat, targ string) bool {
			return len(arch) > 0 && len(plat) == 0 && t.Architecture().Name() == arch
		},

		// If only `plat`` is supplied and the target's platform matches
		func(t target.Target, arch, plat, targ string) bool {
			return len(plat) > 0 && len(arch) == 0 && t.Platform().Name() == plat
		},

		// If both `arch` and `plat` are supplied and match the target
		func(t target.Target, arch, plat, targ string) bool {
			return len(plat) > 0 && len(arch) > 0 && t.Architecture().Name() == arch && t.Platform().Name() == plat
		},
	}

	// Filter `targets` by input arguments `arch`, `plat` and/or `targ`
	for _, t := range targets {
		for _, c := range conditions {
			if !c(t, arch, plat, targ) {
				continue
			}

			selected = append(selected, t)
		}
	}

	return selected
}

func (opts *Build) pull(ctx context.Context, project app.Application, workdir string, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	auths := config.G[config.KraftKit](ctx).Auth

	if _, err := project.Components(ctx); err != nil && project.Template().Name() != "" {
		var packages []pack.Package
		packages, err = packmanager.G(ctx).Catalog(ctx,
			packmanager.WithName(project.Template().Name()),
			packmanager.WithTypes(unikraft.ComponentTypeApp),
			packmanager.WithVersion(project.Template().Version()),
			packmanager.WithCache(!opts.NoCache),
			packmanager.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		)
		if err != nil {
			return err
		}

		if len(packages) == 0 {
			return fmt.Errorf("could not find: %s",
				unikraft.TypeNameVersion(project.Template()),
			)
		} else if len(packages) > 1 {
			return fmt.Errorf("too many options for %s",
				unikraft.TypeNameVersion(project.Template()),
			)
		}

		return packages[0].Pull(
			ctx,
			pack.WithPullWorkdir(workdir),
			// pack.WithPullChecksum(!opts.NoChecksum),
			pack.WithPullCache(!opts.NoCache),
		)
	}

	if project.Template().Name() != "" {
		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		project, err = templateProject.MergeTemplate(ctx, project)
		if err != nil {
			return err
		}
	}

	// Overwrite template with user options
	components, err := project.Components(ctx)
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

		component := component // loop closure
		auths := auths

		p, err := packmanager.G(ctx).Catalog(ctx,
			packmanager.WithName(component.Name()),
			packmanager.WithTypes(component.Type()),
			packmanager.WithVersion(component.Version()),
			packmanager.WithSource(component.Source()),
			packmanager.WithCache(!opts.NoCache),
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
				pack.WithPullWorkdir(workdir),
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

func (opts *Build) BuildCmd(ctxt context.Context, args []string) error {
	var err error
	var workdir string

	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	ctx := ctxt

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	if !app.IsWorkdirInitialized(workdir) {
		return fmt.Errorf("cannot build uninitialized project")
	}

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY
	nameWidth := -1

	// Calculate the width of the longest process name so that we can align the
	// two independent processtrees if we are using "render" mode (aka the fancy
	// mode is enabled).
	if !norender {
		// The longest word is "configuring" (which is 11 characters long), plus
		// additional space characters (2 characters), brackets (2 characters) the
		// name of the project and the target/plat string (which is variable in
		// length).
		for _, targ := range project.Targets() {
			if newLen := len(targ.Name()) + len(target.TargetPlatArchName(targ)) + 15; newLen > nameWidth {
				nameWidth = newLen
			}
		}
	}

	if !opts.NoPull {
		if err := opts.pull(ctx, project, workdir, norender, nameWidth); err != nil {
			return err
		}
	}

	// Filter project targets by any provided CLI options
	selected := FilterTargets(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	if len(selected) == 0 {
		return fmt.Errorf("no targets selected to build")
	}
	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(opts.Fast))
	}

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		if !opts.NoConfigure {
			project.Configure(
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
		}

		if !opts.NoPrepare {
			project.Prepare(
				ctx,
				targ, // Target-specific options
				append(
					mopts,
					make.WithExecOptions(
						exec.WithStdout(log.G(ctx).Writer()),
						exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
					),
				)...,
			)
		}
		project.Build(
			ctx,
			targ, // Target-specific options
			app.WithBuildMakeOptions(append(mopts,
				make.WithExecOptions(
					exec.WithStdout(log.G(ctx).Writer()),
					exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
				),
			)...),
			app.WithBuildLogFile(opts.SaveBuildLog),
		)
	}

	return nil
}

type Pkg struct {
	Architecture string
	Args         string
	Dbg          bool
	Force        bool
	Format       string
	Initrd       string
	Kernel       string
	Name         string
	Output       string
	Platform     string
	Target       string
	Volumes      []string
	WithKConfig  bool
}

func (opts *Pkg) PkgCmd(ctxt context.Context, args []string) error {
	var err error
	var workdir string

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	ctx := ctxt

	// Interpret the project directory
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	// Generate a package for every matching requested target
	for _, targ := range project.Targets() {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ

		switch true {
		case
			// If no arguments are supplied
			len(opts.Target) == 0 &&
				len(opts.Architecture) == 0 &&
				len(opts.Platform) == 0,

			// If the --target flag is supplied and the target name match
			len(opts.Target) > 0 &&
				targ.Name() == opts.Target,

			// If only the --arch flag is supplied and the target's arch matches
			len(opts.Architecture) > 0 &&
				len(opts.Platform) == 0 &&
				targ.Architecture().Name() == opts.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(opts.Platform) > 0 &&
				len(opts.Architecture) == 0 &&
				targ.Platform().Name() == opts.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(opts.Platform) > 0 &&
				len(opts.Architecture) > 0 &&
				targ.Architecture().Name() == opts.Architecture &&
				targ.Platform().Name() == opts.Platform:

			var format pack.PackageFormat
			name := "packaging " + targ.Name()
			if opts.Format != "auto" {
				format = pack.PackageFormat(opts.Format)
			} else if targ.Format().String() != "" {
				format = targ.Format()
			}
			if format.String() != "auto" {
				name += " (" + format.String() + ")"
			}

			cmdShellArgs, err := shellwords.Parse(opts.Args)
			if err != nil {
				return err
			}

			pm := packmanager.G(ctx)

			// Switch the package manager the desired format for this target
			if format != "auto" {
				pm, err = pm.From(format)
				if err != nil {
					return err
				}
			}

			popts := []packmanager.PackOption{
				packmanager.PackArgs(cmdShellArgs...),
				packmanager.PackKConfig(opts.WithKConfig),
				packmanager.PackOutput(opts.Output),
				packmanager.PackInitrd(opts.Initrd),
			}

			if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
				popts = append(popts,
					packmanager.PackWithKernelVersion(ukversion.Value),
				)
			}

			if _, err := pm.Pack(ctx, targ, popts...); err != nil {
				return err
			}

		default:
			continue
		}
	}

	return nil
}

type ProperClean struct{}

func (opts *ProperClean) ProperCleanCmd(ctxt context.Context, args []string) error {
	var err error

	ctx := ctxt
	workdir := ""

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	return project.Properclean(ctx, nil)
}

type Pull struct {
	AllVersions  bool
	Architecture string
	Manager      string
	ForceCache   bool
	NoChecksum   bool
	NoDeps       bool
	Platform     string
	WithDeps     bool
	Workdir      string
}

func (opts *Pull) PullCmd(ctxt context.Context, args []string) error {
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

	ctx := ctxt
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

	var queries []pmQuery

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		workdir = args[0]
		project, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
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
				packmanager.WithCache(!opts.ForceCache),
			)
			if err != nil {
				return err
			}

			if len(packages) == 0 {
				return fmt.Errorf("could not find: %s", unikraft.TypeNameVersion(project.Template()))
			} else if len(packages) > 1 {
				return fmt.Errorf("too many options for %s", unikraft.TypeNameVersion(project.Template()))
			}

			packages[0].Pull(
				ctx,
				pack.WithPullWorkdir(workdir),
				// pack.WithPullChecksum(!opts.NoChecksum),
				// pack.WithPullCache(!opts.NoCache),
			)
		}

		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultKraftfiles(),
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
					packmanager.WithCache(opts.ForceCache),
				},
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else if len(args) > 0 {
		for _, arg := range args {
			pm, compatible, err := pm.IsCompatible(ctx, arg)
			if err != nil || !compatible {
				continue
			}

			queries = append(queries, pmQuery{
				pm: pm,
				query: []packmanager.QueryOption{
					packmanager.WithCache(opts.ForceCache),
					packmanager.WithName(arg),
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
			return p.Pull(
				ctx,
				pack.WithPullWorkdir(workdir),
				pack.WithPullChecksum(!opts.NoChecksum),
				pack.WithPullCache(opts.ForceCache),
			)
		}
	}

	if project != nil {
		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo(ctx))
	}

	return nil
}

type Source struct{}

func (opts *Source) SourceCmd(ctxt context.Context, args []string) error {
	ctx := ctxt

	for _, source := range args {
		_, compatible, err := packmanager.G(ctx).IsCompatible(ctx, source)
		if err != nil {
			return err
		} else if !compatible {
			return errors.New("incompatible package manager")
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

func (opts *Unsource) UnsourceCmd(ctxt context.Context, args []string) error {
	ctx := ctxt
	for _, source := range args {
		_, compatible, err := packmanager.G(ctx).IsCompatible(ctx, source)
		if err != nil {
			return err
		} else if !compatible {
			return errors.New("incompatible package manager")
		}

		manifests := []string{}

		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if source != manifest {
				manifests = append(manifests, manifest)
			}
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

func (opts *Update) UpdateCmd(ctxt context.Context, args []string) error {
	var err error

	ctx := ctxt
	pm := packmanager.G(ctx)

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Manager))
		if err != nil {
			return err
		}
	}

	err = pm.Update(ctx)
	if err != nil {
		return err
	}

	return nil
}

type Set struct {
	Workdir string
}

func (opts *Set) SetCmd(ctxt context.Context, args []string) error {
	var err error

	ctx := ctxt

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

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
		app.WithProjectConfig(confOpts),
	)
	if err != nil {
		return err
	}

	return project.Set(ctx, nil)
}

type Push struct {
	Format string
}

func (opts *Push) Run(ctxt context.Context, args []string) error {
	var err error
	var workdir string

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		workdir = args[0]
	} else {
		workdir = ""
	}

	ctx := ctxt
	ref := ""
	if workdir != "" {
		// Read the kraft yaml specification and get the target name
		project, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		// Get the target name
		ref = project.Name()
	} else {
		// Argument is a reference name
		ref = args[0]
	}

	var pmananger packmanager.PackageManager
	if opts.Format != "auto" {
		pmananger = packmanager.PackageManagers()[pack.PackageFormat(opts.Format)]
		if pmananger == nil {
			return errors.New("invalid package format specified")
		}
	} else {
		pmananger = packmanager.G(ctx)
	}

	if pm, compatible, err := pmananger.IsCompatible(ctx, ref); err == nil && compatible {
		packages, err := pm.Catalog(ctx,
			packmanager.WithCache(true),
			packmanager.WithName(ref),
		)
		if err != nil {
			return err
		}

		if len(packages) == 0 {
			return errors.New("no packages found")
		} else if len(packages) > 1 {
			return errors.New("multiple packages found")
		}

		// Call push if it exists
		// TODO push if it doesn't exist too
		if err := packages[0].Push(ctx); err != nil {
			return err
		}
	} else {
		return err
	}

	return nil
}
