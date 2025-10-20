// A generated module for Sindri functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/frantjc/sindri/.dagger/internal/dagger"
)

type SindriDev struct {
	Source *dagger.Directory
}

func New(
	// +optional
	// +defaultPath="."
	src *dagger.Directory,
) *SindriDev {
	return &SindriDev{
		Source: src,
	}
}

const (
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *SindriDev) Container(
	ctx context.Context,
	// +optional
	// +default="steamapps"
	module string,
) (*dagger.Container, error) {
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
		WithDirectory(".", m.Source.Directory(path.Join("modules", module)), dagger.ContainerWithDirectoryOpts{Owner: owner}).
		WithEntrypoint([]string{"sindri"}), nil
}

func (m *SindriDev) Service(
	ctx context.Context,
	// +optional
	// +default="localhost"
	hostname string,
	// +optional
	// +default="steamapps"
	module string,
) (*dagger.Service, error) {
	keyPair := dag.TLS().Ca().KeyPair(hostname)
	crtPath := home + "/.config/sindri/tls.crt"
	keyPath := home + "/.config/sindri/tls.key"

	container, err := m.Container(ctx, module)
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
				"--tls-key", keyPath,
				"--tls-crt", crtPath,
				"--debug",
			},
		}), nil
}

func (m *SindriDev) Test(
	ctx context.Context,
	// +optional
	// +default="steamapps"
	module string,
	// +optional
	// +default=[
	// "valheim",
	// "corekeeper"
	// ]
	repository []string,
) (*dagger.Container, error) {
	alias := "sindri.dagger.local"
	hostname := fmt.Sprintf("%s:5000", alias)
	caCrtPath := "/usr/share/ca-certificates/dagger.crt"

	svc, err := m.Service(ctx, alias, module)
	if err != nil {
		return nil, err
	}

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

func (m *SindriDev) Coder() *dagger.LLM {
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
				WithMCPServer(
					"lsp",
					dag.Go(dagger.GoOpts{Module: m.Source}).
						Container().
						WithExec([]string{"go", "install", "golang.org/x/tools/gopls@latest"}).
						WithExec([]string{"go", "install", "github.com/isaacphi/mcp-language-server@latest"}).
						AsService(dagger.ContainerAsServiceOpts{
							Args: []string{"mcp-language-server", "--workspace", ".", "--lsp", "gopls"},
						}),
				),
		)
}
