// A generated module for Sindri functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/frantjc/sindri/.dagger/internal/dagger"
	xslices "github.com/frantjc/x/slices"
)

type SindriDev struct {
	Source  *dagger.Directory
	Module  string
	Backend string
}

func New(
	ctx context.Context,
	// +optional
	// +defaultPath="."
	src *dagger.Directory,
	// +optional
	// +default="steamapps"
	module,
	// +optional
	// +default="file:///home/sindri/.cache/sindri"
	backend string,
) (*SindriDev, error) {
	modules, err := src.Entries(ctx, dagger.DirectoryEntriesOpts{Path: "modules"})
	if err != nil {
		return nil, err
	}

	exclude := xslices.Map(
		xslices.Filter(modules, func(m string, _ int) bool {
			return m != "interface/" && m != module+"/"
		}),
		func(m string, _ int) string {
			return path.Join("modules", m)
		},
	)

	return &SindriDev{
		Source:  src.Filter(dagger.DirectoryFilterOpts{Exclude: exclude}),
		Module:  module,
		Backend: backend,
	}, nil
}

const (
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *SindriDev) Container(ctx context.Context) (*dagger.Container, error) {
	version, err := dag.Version(ctx)
	if err != nil {
		return nil, err
	}

	osPlatformVersion, err := dag.DefaultPlatform(ctx)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(osPlatformVersion), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid dagger platform %s", osPlatformVersion)
	}

	platform := parts[1]

	daggerTgz := dag.HTTP(
		fmt.Sprintf(
			"https://github.com/dagger/dagger/releases/download/%s/dagger_%s_linux_%s.tar.gz",
			version, version, platform,
		),
	)

	tmpDaggerTgzPath := "/tmp/dagger.tgz"
	tmpDaggerPath := "/tmp/dagger"

	kubectl := dag.HTTP(
		fmt.Sprintf(
			"https://dl.k8s.io/release/v1.34.0/bin/linux/%s/kubectl",
			platform,
		),
	)

	return dag.Wolfi().
		Container().
		WithExec([]string{"addgroup", "-S", "-g", gid, group}).
		WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}).
		WithEnvVariable("PATH", home+"/.local/bin:$PATH", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithFile(
			home+"/.local/bin/sindri", m.Binary(ctx),
			dagger.ContainerWithFileOpts{Expand: true, Owner: owner, Permissions: 0700}).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", home+"/.local/bin/dagger").
		WithFile(
			"$_EXPERIMENTAL_DAGGER_CLI_BIN",
			dag.Wolfi().
				Container().
				WithFile(tmpDaggerTgzPath, daggerTgz).
				WithExec([]string{
					"tar", "-xzf", tmpDaggerTgzPath, "-C", path.Dir(tmpDaggerPath), path.Base(tmpDaggerPath),
				}).
				File(tmpDaggerPath),
			dagger.ContainerWithFileOpts{Expand: true, Owner: owner, Permissions: 0700},
		).
		WithFile(
			home+"/.local/bin/kubectl", kubectl,
			dagger.ContainerWithFileOpts{Expand: true, Owner: owner, Permissions: 0700},
		).
		WithExec([]string{"chown", "-R", owner, home}).
		WithUser(user).
		WithWorkdir(home+"/.config/sindri/module").
		WithDirectory(".", m.Source.Directory(path.Join("modules", m.Module)), dagger.ContainerWithDirectoryOpts{Owner: owner}).
		WithEntrypoint([]string{"sindri"}), nil
}

func (m *SindriDev) Service(
	ctx context.Context,
	// +optional
	// +default="localhost"
	hostname string,
) (*dagger.Service, error) {
	keyPair := dag.TLS().Ca().KeyPair(hostname)
	crtPath := home + "/.config/sindri/tls.crt"
	keyPath := home + "/.config/sindri/tls.key"

	container, err := m.Container(ctx)
	if err != nil {
		return nil, err
	}

	return container.
		WithMountedCache(home+"/.cache/sindri", dag.CacheVolume("sindri"), dagger.ContainerWithMountedCacheOpts{Owner: owner}).
		WithFile(keyPath, keyPair.Key(), dagger.ContainerWithFileOpts{Permissions: 0400, Owner: owner}).
		WithFile(crtPath, keyPair.Crt(), dagger.ContainerWithFileOpts{Permissions: 0400, Owner: owner}).
		WithExposedPort(5000).
		AsService(dagger.ContainerAsServiceOpts{
			ExperimentalPrivilegedNesting: true,
			UseEntrypoint:                 true,
			Args: []string{
				"--backend", m.Backend,
				"--tls-key", keyPath,
				"--tls-crt", crtPath,
				"--debug",
			},
		}), nil
}

func (m *SindriDev) Test(
	ctx context.Context,
	// +optional
	// +default=[
	// "valheim",
	// "corekeeper"
	// ]
	repository []string,
	// +optional
	// +default="go-containerregistry"
	client string,
) (*dagger.Container, error) {
	alias := "sindri.dagger.local"
	hostname := fmt.Sprintf("%s:5000", alias)
	caCrtPath := "/usr/share/ca-certificates/dagger.crt"

	svc, err := m.Service(ctx, alias)
	if err != nil {
		return nil, err
	}

	// TODO(frantjc): Test containerd client, and maybe others?
	switch client {
	case "go-containerregistry":
		return dag.Go(dagger.GoOpts{
			Module: m.Source.Filter(dagger.DirectoryFilterOpts{
				Include: []string{"go.mod", "go.sum", "e2e/**"},
			}),
		}).
			Container().
			WithFile(caCrtPath, dag.TLS().Ca().Crt()).
			WithExec([]string{
				"sh", "-c", fmt.Sprintf(`cat "%s" >> "/etc/ssl/certs/ca-certificates.crt"`, caCrtPath),
			}).
			WithEnvVariable("SINDRI_TEST_REGISTRY", hostname).
			WithEnvVariable("SINDRI_TEST_REPOSITORIES", strings.Join(repository, ",")).
			WithServiceBinding(alias, svc).
			WithExec([]string{"go", "test", "-race", "-cover", "-timeout", "30m", "./e2e/..."}), nil
	}

	return nil, fmt.Errorf("unknown client %s", client)
}

func (m *SindriDev) Version(ctx context.Context) string {
	version := "0.0.0-unknown"

	ref, err := m.Source.AsGit().LatestVersion().Ref(ctx)
	if err == nil {
		version = strings.TrimPrefix(ref, "refs/tags/v")
	}

	return version
}

func (m *SindriDev) Binary(ctx context.Context) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{".github/**", "e2e/**"},
		}),
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/sindri",
			Ldflags: "-s -w -X main.version=" + m.Version(ctx),
		})
}

func (m *SindriDev) Coder(ctx context.Context) (*dagger.LLM, error) {
	gopls := dag.Go(dagger.GoOpts{Module: m.Source}).
		Container().
		WithExec([]string{"go", "install", "golang.org/x/tools/gopls@latest"})

	instructions, err := gopls.WithExec([]string{"gopls", "mcp", "-instructions"}).Stdout(ctx)
	if err != nil {
		return nil, err
	}

	return dag.Doug().
		Agent(
			dag.LLM().
				WithEnv(
					dag.Env().
						WithCurrentModule().
						WithWorkspace(m.Source.Filter(dagger.DirectoryFilterOpts{
							Exclude: []string{".dagger/**", ".github/**"},
						})),
				).
				WithBlockedFunction("Sindri", "container").
				WithBlockedFunction("Sindri", "service").
				WithBlockedFunction("Sindri", "version").
				WithSystemPrompt(instructions).
				WithMCPServer(
					"gopls",
					gopls.AsService(dagger.ContainerAsServiceOpts{
						Args: []string{"gopls", "mcp"},
					}),
				),
		), nil
}
