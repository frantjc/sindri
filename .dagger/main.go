// A generated module for Sindri functions

package main

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/frantjc/sindri/.dagger/internal/dagger"
)

type SindriDev struct {}

const (
	gid            = "1001"
	uid            = gid
	group          = "sindri"
	user           = group
	owner          = user + ":" + group
	home           = "/home/" + user
	defaultBackend = "file://" + home + "/.cache/sindri"
	defaultModule  = "steamapps"
)

// +check
func (m *SindriDev) Container(
	ctx context.Context,
	workspace *dagger.Workspace,
	// +optional
	module string,
) (*dagger.Container, error) {
	if module == "" {
		module = defaultModule
	}

	version, err := dag.Version(ctx)
	if err != nil {
		return nil, err
	}

	arch, err := dag.Arch().Oci(ctx)
	if err != nil {
		return nil, err
	}

	daggerTgz := dag.HTTP(
		fmt.Sprintf(
			"https://github.com/dagger/dagger/releases/download/%s/dagger_%s_linux_%s.tar.gz",
			version, version, arch,
		),
	)

	kubectl := dag.HTTP(
		fmt.Sprintf(
			"https://dl.k8s.io/release/v1.34.3/bin/linux/%s/kubectl",
			arch,
		),
	)

	return dag.Wolfi().
		Container().
		WithExec([]string{"addgroup", "-S", "-g", gid, group}).
		WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}).
		WithEnvVariable("PATH", home+"/.local/bin:$PATH", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithFile(
			home+"/.local/bin/sindri", dag.SindriDev().Binary(),
			dagger.ContainerWithFileOpts{Expand: true, Owner: owner, Permissions: 0700}).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", home+"/.local/bin/dagger").
		WithFile(
			"$_EXPERIMENTAL_DAGGER_CLI_BIN",
			dag.Archive().Untar(daggerTgz).File("dagger"),
			dagger.ContainerWithFileOpts{Expand: true, Owner: owner, Permissions: 0700},
		).
		WithFile(
			home+"/.local/bin/kubectl", kubectl,
			dagger.ContainerWithFileOpts{Expand: true, Owner: owner, Permissions: 0700},
		).
		WithExec([]string{"chown", "-R", owner, home}).
		WithUser(user).
		WithWorkdir(home+"/.config/sindri/module").
		WithDirectory(".", workspace.Directory(path.Join("modules", module)), dagger.ContainerWithDirectoryOpts{Owner: owner}).
		WithEntrypoint([]string{"sindri"}), nil
}

func (m *SindriDev) Service(
	ctx context.Context,
	workspace *dagger.Workspace,
	// +optional
	backend,
	// +optional
	module string,
) (*dagger.Service, error) {
	// NB: Not using +default pragma because it does not get used when
	// other methods in the module call the method with the pragma.
	if backend == "" {
		backend = defaultBackend
	}

	u, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}

	container, err := m.Container(ctx, workspace, module)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "file" {
		container = container.
			WithMountedCache(path.Join(u.Host, u.Path), dag.CacheVolume("sindri"), dagger.ContainerWithMountedCacheOpts{Owner: owner})

		q := u.Query()
		if noTmpDir, _ := strconv.ParseBool(q.Get("no_tmp_dir")); !noTmpDir {
			q.Set("no_tmp_dir", "1")
			u.RawQuery = q.Encode()
		}
	}

	return container.
		WithExposedPort(5000).
		AsService(dagger.ContainerAsServiceOpts{
			ExperimentalPrivilegedNesting: true,
			UseEntrypoint:                 true,
			Args: []string{
				"--backend", u.String(),
				"--debug",
			},
		}), nil
}

func (m *SindriDev) Version(
	ctx context.Context, 
	workspace *dagger.Workspace,
) string {
	version := "v0.0.0-unknown"

	source := workspace.Directory(".")
	gitRef := source.AsGit().LatestVersion()

	

	if ref, err := gitRef.Ref(ctx); err == nil {
		version = strings.TrimPrefix(ref, "refs/tags/")
	}

	if latestVersionCommit, err := gitRef.Commit(ctx); err == nil {
		if headCommit, err := source.AsGit().Head().Commit(ctx); err == nil {
			if headCommit != latestVersionCommit {
				if len(headCommit) > 7 {
					headCommit = headCommit[:7]
				}
				version += "-" + headCommit
			}
		}
	}

	if empty, _ := source.AsGit().Uncommitted().IsEmpty(ctx); !empty {
		version += "+dirty"
	}

	return version
}

func (m *SindriDev) Tag(
	ctx context.Context,
	workspace *dagger.Workspace,
) string {
	before, _, _ := strings.Cut(strings.TrimPrefix(m.Version(ctx, workspace), "v"), "+")
	return before
}

func (m *SindriDev) Binary(
	ctx context.Context,
	workspace *dagger.Workspace,
) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Workspace: workspace,
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/sindri",
			Ldflags: "-s -w -X main.version=" + m.Version(ctx, workspace),
		})
}

// +check
func (m *SindriDev) Test(
	ctx context.Context,
	workspace *dagger.Workspace,
) error {
	tags := []string{
		"dagger",
		"git",
		"steamapps",
		"wolfi",
	}
	return dag.Go(dagger.GoOpts{
		Workspace: workspace,
	}).
		Test(ctx, dagger.GoTestOpts{
			Race: true,
			Tags: tags,
		})
}
