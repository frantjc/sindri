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
	xslices "github.com/frantjc/x/slices"
)

type SindriDev struct {
	Source *dagger.Directory
}

func New(
	ctx context.Context,
	// +optional
	// +defaultPath="."
	src *dagger.Directory,
) (*SindriDev, error) {
	return &SindriDev{
		Source: src,
	}, nil
}

func (m *SindriDev) Fmt(
	ctx context.Context,
	// +optional
	check bool,
) (*dagger.Changeset, error) {
	goModules := []string{
		".dagger/",
		"modules/git/",
		"modules/interface/",
		"modules/steamapps/",
		"modules/steamapps/abioticfactor/",
		"modules/steamapps/astroneer/",
		"modules/steamapps/corekeeper/",
		"modules/steamapps/enshrouded/",
		"modules/steamapps/palworld/",
		"modules/steamapps/satisfactory/",
		"modules/steamapps/valheim/",
		"modules/wolfi/",
	}

	root := dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: goModules,
		}),
	}).
		Container().
		WithExec([]string{"go", "fmt", "./..."}).
		Directory(".")

	for _, module := range goModules {
		root = root.WithDirectory(
			module,
			dag.Go(dagger.GoOpts{
				Module: m.Source.Directory(module).Filter(dagger.DirectoryFilterOpts{
					Exclude: xslices.Filter(goModules, func(m string, _ int) bool {
						return strings.HasPrefix(m, module)
					}),
				}),
			}).
				Container().
				WithExec([]string{"go", "fmt", "./..."}).
				Directory("."),
		)
	}

	changeset := root.Changes(m.Source)

	if check {
		if empty, err := changeset.IsEmpty(ctx); err != nil {
			return nil, err
		} else if !empty {
			return nil, fmt.Errorf("source is not formatted")
		}
	}

	return changeset, nil
}

const (
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
	defaultBackend = "file://" + home + "/.cache/sindri"
	defaultModule  = "steamapps"
)

func (m *SindriDev) Container(
	ctx context.Context,
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

	container, err := m.Container(ctx, module)
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

	keyPair := dag.TLS().Ca().KeyPair(hostname)
	crtPath := home + "/.config/sindri/tls.crt"
	keyPath := home + "/.config/sindri/tls.key"

	return container.
		WithFile(keyPath, keyPair.Key(), dagger.ContainerWithFileOpts{Permissions: 0400, Owner: owner}).
		WithFile(crtPath, keyPair.Crt(), dagger.ContainerWithFileOpts{Permissions: 0400, Owner: owner}).
		WithExposedPort(5000).
		AsService(dagger.ContainerAsServiceOpts{
			ExperimentalPrivilegedNesting: true,
			UseEntrypoint:                 true,
			Args: []string{
				"--backend", u.String(),
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
	client,
	// +optional
	backend,
	// +optional
	module string,
) (*dagger.Container, error) {
	alias := "sindri.dagger.local"
	hostname := fmt.Sprintf("%s:5000", alias)
	caCrtPath := "/usr/share/ca-certificates/dagger.crt"

	svc, err := m.Service(ctx, alias, backend, module)
	if err != nil {
		return nil, err
	}

	// TODO(frantjc): Test containerd client, and maybe others?
	switch client {
	case "go-containerregistry":
		return dag.Go(dagger.GoOpts{
			Module: m.Source.Filter(dagger.DirectoryFilterOpts{
				Include: []string{"go.mod", "go.sum", "e2e/"},
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
	version := "v0.0.0-unknown"

	ref, err := m.Source.AsGit().LatestVersion().Ref(ctx)
	if err == nil {
		version = strings.TrimPrefix(ref, "refs/tags/")
	}

	if empty, _ := m.Source.AsGit().Uncommitted().IsEmpty(ctx); !empty {
		version += "*"
	}

	return version
}

func (m *SindriDev) Tag(ctx context.Context) string {
	return strings.TrimSuffix(strings.TrimPrefix(m.Version(ctx), "v"), "*")
}

func (m *SindriDev) Binary(ctx context.Context) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{".github/", "e2e/"},
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
							Exclude: []string{".dagger/", ".github/"},
						})),
				).
				WithBlockedFunction("Sindri", "container").
				WithBlockedFunction("Sindri", "service").
				WithBlockedFunction("Sindri", "tag").
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
