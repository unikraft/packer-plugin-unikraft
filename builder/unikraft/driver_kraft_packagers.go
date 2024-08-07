// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package unikraft

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/multiselect"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

type packager interface {
	// String implements fmt.Stringer and returns the name of the implementing
	// builder.
	fmt.Stringer

	// Packagable determines whether the provided input is packagable by the
	// current implementation.
	Packagable(context.Context, *Pkg, ...string) (bool, error)

	// Pack performs the packaging based on the determined implementation.
	Pack(context.Context, *Pkg, ...string) ([]pack.Package, error)
}

// packagers is the list of built-in packagers which are checked
// sequentially for capability.  The first to test positive via Packagable
// is used with the controller.
func packagers() []packager {
	return []packager{
		&packagerKraftfileUnikraft{},
		&packagerKraftfileRuntime{},
		&packagerCliKernel{},
		&packagerDockerfile{},
	}
}

type packagerKraftfileUnikraft struct{}

// String implements fmt.Stringer.
func (p *packagerKraftfileUnikraft) String() string {
	return "kraftfile-unikraft"
}

// Buildable implements packager.
func (p *packagerKraftfileUnikraft) Packagable(ctx context.Context, opts *Pkg, args ...string) (bool, error) {
	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Unikraft(ctx) == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	return true, nil
}

// Build implements packager.
func (p *packagerKraftfileUnikraft) Pack(ctx context.Context, opts *Pkg, args ...string) ([]pack.Package, error) {
	var err error

	selected := opts.Project.Targets()
	if len(opts.Target) > 0 || len(opts.Architecture) > 0 || len(opts.Platform) > 0 {
		selected = target.Filter(opts.Project.Targets(), opts.Architecture, opts.Platform, opts.Target)
	}

	if len(selected) > 1 && !config.G[config.KraftKit](ctx).NoPrompt {
		// Remove targets which do not have a compiled kernel.
		targets := slices.DeleteFunc(opts.Project.Targets(), func(targ target.Target) bool {
			_, err := os.Stat(targ.Kernel())
			return err != nil
		})

		if len(targets) == 0 {
			return nil, fmt.Errorf("no targets with a compiled kernel found")
		} else if len(targets) == 1 {
			selected = targets
		} else {
			selected, err = multiselect.MultiSelect[target.Target]("select built kernel to package", targets...)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("nothing selected to package")
	}

	i := 0

	var result []pack.Package

	for _, targ := range selected {
		var cmds []string
		var envs []string
		rootfs := opts.Rootfs

		// Reset the rootfs, such that it is not packaged as an initrd if it is
		// already embedded inside of the kernel.
		if opts.Project.KConfig().AnyYes(
			"CONFIG_LIBVFSCORE_ROOTFS_EINITRD", // Deprecated
			"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
			"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
		) {
			rootfs = ""
		} else {
			if rootfs, cmds, envs, err = BuildRootfs(ctx, opts.Workdir, rootfs, false, targ.Architecture().String()); err != nil {
				return nil, fmt.Errorf("could not build rootfs: %w", err)
			}
		}

		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		baseopts := opts.packopts

		if envs != nil {
			opts.Env = append(opts.Env, envs...)
		}

		// If no arguments have been specified, use the ones which are default and
		// that have been included in the package.
		if len(opts.Args) == 0 {
			if len(opts.Project.Command()) > 0 {
				opts.Args = opts.Project.Command()
			} else if len(targ.Command()) > 0 {
				opts.Args = targ.Command()
			} else if cmds != nil {
				opts.Args = cmds
			}
		}

		cmdShellArgs, err := shellwords.Parse(strings.Join(opts.Args, " "))
		if err != nil {
			return nil, err
		}

		labels := opts.Project.Labels()
		if len(opts.Labels) > 0 {
			for _, label := range opts.Labels {
				kv := strings.SplitN(label, "=", 2)
				if len(kv) != 2 {
					return nil, fmt.Errorf("invalid label format: %s", label)
				}

				labels[kv[0]] = kv[1]
			}
		}

		// When i > 0, we have already applied the merge strategy.  Now, for all
		// targets, we actually do wish to merge these because they are part of
		// the same execution lifecycle.
		if i > 0 {
			baseopts = []packmanager.PackOption{
				packmanager.PackMergeStrategy(packmanager.StrategyMerge),
			}
		}

		popts := append(baseopts,
			packmanager.PackArgs(cmdShellArgs...),
			packmanager.PackInitrd(rootfs),
			packmanager.PackKConfig(!opts.NoKConfig),
			packmanager.PackName(opts.Name),
			packmanager.PackOutput(opts.Output),
			packmanager.PackLabels(labels),
		)

		if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
			popts = append(popts,
				packmanager.PackWithKernelVersion(ukversion.Value),
			)
		}

		envs = opts.aggregateEnvs()
		if len(envs) > 0 {
			popts = append(popts, packmanager.PackWithEnvs(envs))
		} else if len(opts.Env) > 0 {
			popts = append(popts, packmanager.PackWithEnvs(opts.Env))
		}

		more, err := opts.pm.Pack(ctx, targ, popts...)
		if err != nil {
			return nil, err
		}

		result = append(result, more...)

		i++
	}

	if len(result) == 0 {
		switch true {
		case len(opts.Target) > 0:
			return nil, fmt.Errorf("no matching targets found for: %s", opts.Target)
		case len(opts.Architecture) > 0 && len(opts.Platform) == 0:
			return nil, fmt.Errorf("no matching targets found for architecture: %s", opts.Architecture)
		case len(opts.Architecture) == 0 && len(opts.Platform) > 0:
			return nil, fmt.Errorf("no matching targets found for platform: %s", opts.Platform)
		default:
			return nil, fmt.Errorf("no matching targets found for: %s/%s", opts.Platform, opts.Architecture)
		}
	}

	return result, nil
}

type packagerKraftfileRuntime struct{}

// String implements fmt.Stringer.
func (p *packagerKraftfileRuntime) String() string {
	return "kraftfile-runtime"
}

// Packagable implements packager.
func (p *packagerKraftfileRuntime) Packagable(ctx context.Context, opts *Pkg, args ...string) (bool, error) {
	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Runtime() == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	return true, nil
}

// Pack implements packager.
func (p *packagerKraftfileRuntime) Pack(ctx context.Context, opts *Pkg, args ...string) ([]pack.Package, error) {
	var err error
	var targ target.Target
	var runtimeName string

	if len(opts.Runtime) > 0 {
		runtimeName = opts.Runtime
	} else {
		runtimeName = opts.Project.Runtime().Name()
	}

	if opts.Platform == "kraftcloud" || (opts.Project.Runtime().Platform() != nil && opts.Project.Runtime().Platform().Name() == "kraftcloud") {
		runtimeName = rewrapAsKraftCloudPackage(runtimeName)
	}

	targets := opts.Project.Targets()
	qopts := []packmanager.QueryOption{
		packmanager.WithName(runtimeName),
		packmanager.WithVersion(opts.Project.Runtime().Version()),
	}

	if len(targets) == 1 {
		targ = targets[0]
	} else if len(targets) > 1 {
		// Filter project targets by any provided CLI options
		targets = target.Filter(
			targets,
			opts.Architecture,
			opts.Platform,
			opts.Target,
		)

		switch {
		case len(targets) == 0:
			return nil, fmt.Errorf("could not detect any project targets based on plat=\"%s\" arch=\"%s\"", opts.Platform, opts.Architecture)

		case len(targets) == 1:
			targ = targets[0]

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
			return nil, fmt.Errorf("could not determine what to run based on provided CLI arguments")

		default:
			targ, err = target.Select(targets)
			if err != nil {
				return nil, fmt.Errorf("could not select target: %v", err)
			}
		}
	}

	var selected *pack.Package
	var packs []pack.Package
	var kconfigs []string

	if targ != nil {
		for _, kc := range targ.KConfig() {
			kconfigs = append(kconfigs, kc.String())
		}

		opts.Platform = targ.Platform().Name()
		opts.Architecture = targ.Architecture().Name()
	}

	qopts = append(qopts,
		packmanager.WithArchitecture(opts.Architecture),
		packmanager.WithPlatform(opts.Platform),
		packmanager.WithKConfig(kconfigs),
	)

	packs, err = opts.pm.Catalog(ctx, append(qopts, packmanager.WithRemote(false))...)
	if err != nil {
		return nil, fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		// Try again with a remote update request.  Save this to qopts in case we
		// need to call `Catalog` again.
		packs, err = opts.pm.Catalog(ctx, append(qopts, packmanager.WithRemote(true))...)
		if err != nil {
			return nil, fmt.Errorf("could not query catalog: %w", err)
		}
	}
	if err != nil {
		return nil, err
	}

	if len(packs) == 0 {
		if len(opts.Platform) > 0 && len(opts.Architecture) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' (%s/%s)",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
				opts.Platform,
				opts.Architecture,
			)
		} else if len(opts.Architecture) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' architecture",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
				opts.Architecture,
			)
		} else if len(opts.Platform) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' platform",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
				opts.Platform,
			)
		} else {
			return nil, fmt.Errorf(
				"could not find runtime %s:%s",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
			)
		}
	} else if len(packs) == 1 {
		selected = &packs[0]
	} else if len(packs) > 1 {
		return nil, fmt.Errorf("multiple runtime packages found: %v", packs)
	}

	runtime := *selected
	pulled, _, _ := runtime.PulledAt(ctx)

	// Temporarily save the runtime package.
	if err := runtime.Save(ctx); err != nil {
		return nil, fmt.Errorf("could not save runtime package: %w", err)
	}

	// Remove the cached runtime package reference if it was not previously
	// pulled.
	if !pulled && opts.NoPull {
		defer func() {
			if err := runtime.Delete(ctx); err != nil {
				log.G(ctx).Debugf("could not delete intermediate runtime package: %s", err.Error())
			}
		}()
	}

	if !pulled && !opts.NoPull {
		popts := []pack.PullOption{}
		err := runtime.Pull(
			ctx,
			popts...,
		)
		if err != nil {
			return nil, fmt.Errorf("could not pull runtime package: %w", err)
		}
	}

	// Create a temporary directory we can use to store the artifacts from
	// pulling and extracting the identified package.
	tempDir, err := os.MkdirTemp("", "kraft-pkg-")
	if err != nil {
		return nil, fmt.Errorf("could not create temporary directory: %w", err)
	}

	defer func() {
		os.RemoveAll(tempDir)
	}()

	// Crucially, the catalog should return an interface that also implements
	// target.Target.  This demonstrates that the implementing package can
	// resolve application kernels.
	targ, ok := runtime.(target.Target)
	if !ok {
		return nil, fmt.Errorf("package does not convert to target")
	}

	var cmds []string
	var envs []string
	if opts.Rootfs, cmds, envs, err = BuildRootfs(ctx, opts.Workdir, opts.Rootfs, false, targ.Architecture().String()); err != nil {
		return nil, fmt.Errorf("could not build rootfs: %w", err)
	}

	if envs != nil {
		opts.Env = append(opts.Env, envs...)
	}

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(opts.Args) == 0 {
		if len(opts.Project.Command()) > 0 {
			opts.Args = opts.Project.Command()
		} else if len(targ.Command()) > 0 {
			opts.Args = targ.Command()
		} else if cmds != nil {
			opts.Args = cmds
		}
	}

	args = []string{}

	// Only parse arguments if they have been provided.
	if len(opts.Args) > 0 {
		args, err = shellwords.Parse(fmt.Sprintf("'%s'", strings.Join(opts.Args, "' '")))
		if err != nil {
			return nil, err
		}
	}

	labels := opts.Project.Labels()
	if len(opts.Labels) > 0 {
		for _, label := range opts.Labels {
			kv := strings.SplitN(label, "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid label format: %s", label)
			}

			labels[kv[0]] = kv[1]
		}
	}

	var result []pack.Package

	popts := append(opts.packopts,
		packmanager.PackArgs(args...),
		packmanager.PackInitrd(opts.Rootfs),
		packmanager.PackKConfig(!opts.NoKConfig),
		packmanager.PackName(opts.Name),
		packmanager.PackOutput(opts.Output),
		packmanager.PackLabels(labels),
	)

	if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
		popts = append(popts,
			packmanager.PackWithKernelVersion(ukversion.Value),
		)
	}

	envs = opts.aggregateEnvs()
	if len(envs) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(envs))
	} else if len(opts.Env) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(opts.Env))
	}

	more, err := opts.pm.Pack(ctx, targ, popts...)
	if err != nil {
		return nil, err
	}

	result = append(result, more...)

	if err != nil {
		return nil, err
	}

	return result, nil
}

type packagerCliKernel struct{}

// String implements fmt.Stringer.
func (p *packagerCliKernel) String() string {
	return "cli-kernel"
}

// Packagable implements packager.
func (p *packagerCliKernel) Packagable(ctx context.Context, opts *Pkg, args ...string) (bool, error) {
	if len(opts.Kernel) > 0 && len(opts.Architecture) > 0 && len(opts.Platform) > 0 {
		return true, nil
	}

	if len(opts.Kernel) > 0 {
		log.G(ctx).Warn("--kernel flag set but must be used in conjunction with -m|--arch and -p|--plat")
	}

	return false, fmt.Errorf("cannot package without path to -k|-kernel, -m|--arch and -p|--plat")
}

// Pack implements packager.
func (p *packagerCliKernel) Pack(ctx context.Context, opts *Pkg, args ...string) ([]pack.Package, error) {
	var err error

	ac := arch.NewArchitectureFromOptions(
		arch.WithName(opts.Architecture),
	)
	pc := plat.NewPlatformFromOptions(
		plat.WithName(opts.Platform),
	)

	targ := target.NewTargetFromOptions(
		target.WithArchitecture(ac),
		target.WithPlatform(pc),
		target.WithKernel(opts.Kernel),
		target.WithCommand(opts.Args),
	)

	var cmds []string
	var envs []string
	if opts.Rootfs, cmds, envs, err = BuildRootfs(ctx, opts.Workdir, opts.Rootfs, false, targ.Architecture().String()); err != nil {
		return nil, fmt.Errorf("could not build rootfs: %w", err)
	}

	if len(opts.Args) == 0 && cmds != nil {
		opts.Args = cmds
	}

	if envs != nil {
		opts.Env = append(opts.Env, envs...)
	}

	var result []pack.Package

	popts := append(opts.packopts,
		packmanager.PackArgs(opts.Args...),
		packmanager.PackInitrd(opts.Rootfs),
		packmanager.PackKConfig(!opts.NoKConfig),
		packmanager.PackName(opts.Name),
		packmanager.PackOutput(opts.Output),
	)

	envs = opts.aggregateEnvs()
	if len(envs) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(envs))
	} else if len(opts.Env) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(opts.Env))
	}

	more, err := opts.pm.Pack(ctx, targ, popts...)
	if err != nil {
		return nil, err
	}

	result = append(result, more...)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type packagerDockerfile struct{}

// String implements fmt.Stringer.
func (p *packagerDockerfile) String() string {
	return "dockerfile"
}

// Packagable implements packager.
func (p *packagerDockerfile) Packagable(ctx context.Context, opts *Pkg, args ...string) (bool, error) {
	if opts.Project == nil {
		// Do not capture the the project is not initialized, as we can still build
		// the unikernel using the Dockerfile provided with the `--rootfs`.
		_ = opts.initProject(ctx)
	}

	if opts.Project != nil && opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	// TODO(nderjung): This is a very naiive check and should be improved,
	// potentially using an external library which parses the Dockerfile syntax.
	// In most cases, however, the Dockerfile is usually named `Dockerfile`.
	if !strings.Contains(strings.ToLower(opts.Rootfs), "dockerfile") {
		return false, fmt.Errorf("%s is not a Dockerfile", opts.Rootfs)
	}

	return true, nil
}

// Build implements packager.
func (p *packagerDockerfile) Pack(ctx context.Context, opts *Pkg, args ...string) ([]pack.Package, error) {
	return (&packagerKraftfileRuntime{}).Pack(ctx, opts, args...)
}
