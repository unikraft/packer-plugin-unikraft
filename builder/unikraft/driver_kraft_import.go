// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package unikraft

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
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
	Platform     string
	SaveBuildLog string
	Target       string
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
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	if !app.IsWorkdirInitialized(workdir) {
		return fmt.Errorf("cannot build uninitialized project! start with: ukbuild init")
	}

	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem

	_, err = project.Components()
	if err != nil && project.Template().Name() != "" {
		var packages []pack.Package
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s/%s:%s...", project.Template().Type(), project.Template().Name(), project.Template().Version()), "",
			func(ctx context.Context) error {
				packages, err = packmanager.G(ctx).Catalog(ctx, packmanager.CatalogQuery{
					Name:    project.Template().Name(),
					Types:   []unikraft.ComponentType{unikraft.ComponentTypeApp},
					Version: project.Template().Version(),
					NoCache: opts.NoCache,
				})
				if err != nil {
					return err
				}

				if len(packages) == 0 {
					return fmt.Errorf("could not find: %s", project.Template().Name())
				} else if len(packages) > 1 {
					return fmt.Errorf("too many options for %s", project.Template().Name())
				}

				return nil
			},
		)

		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
			},
			search,
		)
		if err != nil {
			return err
		}

		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}

		proc := paraprogress.NewProcess(
			fmt.Sprintf("pulling %s", packages[0].Options().TypeNameVersion()),
			func(ctx context.Context, w func(progress float64)) error {
				return packages[0].Pull(
					ctx,
					pack.WithPullProgressFunc(w),
					pack.WithPullWorkdir(workdir),
					// pack.WithPullChecksum(!opts.NoChecksum),
					pack.WithPullCache(!opts.NoCache),
				)
			},
		)

		processes = append(processes, proc)

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	if project.Template().Name() != "" {
		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		project = templateProject.MergeTemplate(project)
	}

	// Overwrite template with user options
	components, err := project.Components()
	if err != nil {
		return err
	}
	for _, component := range components {
		component := component // loop closure

		searches = append(searches, processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s/%s:%s...", component.Type(), component.Component().Name, component.Component().Version), "",
			func(ctx context.Context) error {
				p, err := packmanager.G(ctx).Catalog(ctx, packmanager.CatalogQuery{
					Name: component.Name(),
					Types: []unikraft.ComponentType{
						component.Type(),
					},
					Version: component.Version(),
					NoCache: opts.NoCache,
				})
				if err != nil {
					return err
				}

				if len(p) == 0 {
					return fmt.Errorf("could not find: %s", component.Component().Name)
				} else if len(p) > 1 {
					return fmt.Errorf("too many options for %s", component.Component().Name)
				}

				missingPacks = append(missingPacks, p...)
				return nil
			},
		))
	}

	if len(searches) > 0 {
		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
			},
			searches...,
		)
		if err != nil {
			return err
		}

		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}
	}

	if len(missingPacks) > 0 {
		for _, p := range missingPacks {
			if p.Options() == nil {
				return fmt.Errorf("unexpected error occurred please try again")
			}
			p := p // loop closure
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", p.Options().TypeNameVersion()),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						// pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
					)
				},
			))
		}

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	processes = []*paraprogress.Process{} // reset

	var selected target.Targets
	targets, err := project.Targets()
	if err != nil {
		return err
	}

	// Filter the targets by CLI selection
	for _, targ := range targets {
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
				targ.Architecture.Name() == opts.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(opts.Platform) > 0 &&
				len(opts.Architecture) == 0 &&
				targ.Platform.Name() == opts.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(opts.Platform) > 0 &&
				len(opts.Architecture) > 0 &&
				targ.Architecture.Name() == opts.Architecture &&
				targ.Platform.Name() == opts.Platform:

			selected = append(selected, targ)

		default:
			continue
		}
	}

	if len(selected) == 0 {
		log.G(ctx).Info("no selected to build")
		return nil
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
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("configuring %s (%s)", targ.Name(), targ.ArchPlatString()),
				func(ctx context.Context, w func(progress float64)) error {
					return project.DefConfig(
						ctx,
						&targ, // Target-specific options
						nil,   // No extra configuration options
						make.WithProgressFunc(w),
						make.WithSilent(true),
						make.WithExecOptions(
							exec.WithStdin(iostreams.G(ctx).In),
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
						),
					)
				},
			))
		}

		if !opts.NoPrepare {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("preparing %s (%s)", targ.Name(), targ.ArchPlatString()),
				func(ctx context.Context, w func(progress float64)) error {
					return project.Prepare(
						ctx,
						&targ, // Target-specific options
						append(
							mopts,
							make.WithProgressFunc(w),
							make.WithExecOptions(
								exec.WithStdout(log.G(ctx).Writer()),
								exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
							),
						)...,
					)
				},
			))
		}

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("building %s (%s)", targ.Name(), targ.ArchPlatString()),
			func(ctx context.Context, w func(progress float64)) error {
				return project.Build(
					ctx,
					&targ, // Target-specific options
					app.WithBuildProgressFunc(w),
					app.WithBuildMakeOptions(append(mopts,
						make.WithExecOptions(
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
						),
					)...),
					app.WithBuildLogFile(opts.SaveBuildLog),
				)
			},
		))
	}

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		// Disable parallelization as:
		//  - The first process may be pulling the container image, which is
		//    necessary for the subsequent build steps;
		//  - The Unikraft build system can re-use compiled files from previous
		//    compilations (if the architecture does not change).
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(norender),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	return paramodel.Start()
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
	NoCache      bool
	NoChecksum   bool
	NoDeps       bool
	Platform     string
	WithDeps     bool
	Workdir      string
}

func (opts *Pull) PullCmd(ctxt context.Context, args []string) error {
	var err error
	var project *app.ApplicationConfig
	var processes []*paraprogress.Process
	var queries []packmanager.CatalogQuery

	query := ""
	if len(args) > 0 {
		query = strings.Join(args, " ")
	}

	if len(query) == 0 {
		query, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	workdir := opts.Workdir
	ctx := ctxt
	pm := packmanager.G(ctx)
	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(opts.Manager)
		if err != nil {
			return err
		}
	}

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(query); err == nil && f.IsDir() {
		workdir = query
		project, err := app.NewProjectFromOptions(
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		_, err = project.Components()
		if err != nil {
			// Pull the template from the package manager
			var packages []pack.Package
			search := processtree.NewProcessTreeItem(
				fmt.Sprintf("finding %s/%s:%s...", project.Template().Type(), project.Template().Name(), project.Template().Version()), "",
				func(ctx context.Context) error {
					packages, err = pm.Catalog(ctx, packmanager.CatalogQuery{
						Name:    project.Template().Name(),
						Types:   []unikraft.ComponentType{unikraft.ComponentTypeApp},
						Version: project.Template().Version(),
						NoCache: opts.NoCache,
					})
					if err != nil {
						return err
					}

					if len(packages) == 0 {
						return fmt.Errorf("could not find: %s", project.Template().Name())
					} else if len(packages) > 1 {
						return fmt.Errorf("too many options for %s", project.Template().Name())
					}
					return nil
				},
			)

			treemodel, err := processtree.NewProcessTree(
				ctx,
				[]processtree.ProcessTreeOption{
					processtree.IsParallel(parallel),
					processtree.WithRenderer(norender),
					processtree.WithFailFast(true),
				},
				[]*processtree.ProcessTreeItem{search}...,
			)
			if err != nil {
				return err
			}

			if err := treemodel.Start(); err != nil {
				return fmt.Errorf("could not complete search: %v", err)
			}

			proc := paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", packages[0].Options().TypeNameVersion()),
				func(ctx context.Context, w func(progress float64)) error {
					return packages[0].Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						// pack.WithPullChecksum(!opts.NoChecksum),
						// pack.WithPullCache(!opts.NoCache),
					)
				},
			)

			processes = append(processes, proc)

			paramodel, err := paraprogress.NewParaProgress(
				ctx,
				processes,
				paraprogress.IsParallel(parallel),
				paraprogress.WithRenderer(norender),
				paraprogress.WithFailFast(true),
			)
			if err != nil {
				return err
			}

			if err := paramodel.Start(); err != nil {
				return fmt.Errorf("could not pull all components: %v", err)
			}
		}

		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		project = templateProject.MergeTemplate(project)
		// List the components
		components, err := project.Components()
		if err != nil {
			return err
		}
		for _, c := range components {
			queries = append(queries, packmanager.CatalogQuery{
				Name:    c.Name(),
				Version: c.Version(),
				Types:   []unikraft.ComponentType{c.Type()},
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else {
		for _, c := range strings.Split(query, " ") {
			query := packmanager.CatalogQuery{}
			t, n, v, err := unikraft.GuessTypeNameVersion(c)
			if err != nil {
				continue
			}

			if t != unikraft.ComponentTypeUnknown {
				query.Types = append(query.Types, t)
			}

			if len(n) > 0 {
				query.Name = n
			}

			if len(v) > 0 {
				query.Version = v
			}

			queries = append(queries, query)
		}
	}

	for _, c := range queries {
		next, err := pm.Catalog(ctx, c)
		if err != nil {
			return err
		}

		if len(next) == 0 {
			log.G(ctx).Warnf("could not find %s", c.String())
			continue
		}

		for _, p := range next {
			p := p
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", p.Options().TypeNameVersion()),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
					)
				},
			))
		}
	}

	model, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		paraprogress.IsParallel(parallel),
		paraprogress.WithRenderer(norender),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	if project != nil {
		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo())
	}

	return nil
}

type Source struct{}

func (opts *Source) SourceCmd(ctxt context.Context, args []string) error {
	var err error

	source := ""
	if len(args) > 0 {
		source = args[0]
	}

	ctx := ctxt
	pm := packmanager.G(ctx)

	pm, err = pm.IsCompatible(ctx, source)
	if err != nil {
		return err
	}

	if err = pm.AddSource(ctx, source); err != nil {
		return err
	}

	return nil
}

type Unsource struct{}

func (opts *Unsource) UnsourceCmd(ctxt context.Context, args []string) error {
	var err error
	source := ""

	if len(args) > 0 {
		source = args[0]
	}

	ctx := ctxt
	pm := packmanager.G(ctx)

	pm, err = pm.IsCompatible(ctx, source)
	if err != nil {
		return err
	}

	return pm.RemoveSource(ctx, source)
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
		pm, err = pm.From(opts.Manager)
		if err != nil {
			return err
		}
	}

	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			// processtree.WithVerb("Updating"),
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
		},
		[]*processtree.ProcessTreeItem{
			processtree.NewProcessTreeItem(
				"Updating...",
				"",
				func(ctx context.Context) error {
					return pm.Update(ctx)
				},
			),
		}...,
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
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
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
		app.WithProjectConfig(confOpts),
	)
	if err != nil {
		return err
	}

	return project.Set(ctx, nil)
}

type Unset struct {
	Workdir string
}

func (opts *Unset) UnsetCmd(ctxt context.Context, args []string) error {
	var err error

	ctx := ctxt

	workdir := ""
	confOpts := []string{}

	// Skip if nothing can be unset
	if len(args) == 0 {
		return fmt.Errorf("no options to unset")
	}

	// Set the working directory
	if opts.Workdir != "" {
		workdir = opts.Workdir
	} else {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	for _, arg := range args {
		confOpts = append(confOpts, arg+"=n")
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
		app.WithProjectWorkdir(workdir),
		// app.WithProjectDefaultConfigPath(),
		app.WithProjectConfig(confOpts),
	)
	if err != nil {
		return err
	}

	return project.Unset(ctx, nil)
}
