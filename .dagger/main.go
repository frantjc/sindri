// A generated module for Sindri functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/frantjc/sindri/.dagger/internal/dagger"
)

type Sindri struct {
	Source *dagger.Directory
}

func New(
	// +optional
	// +defaultPath="."
	src *dagger.Directory,
) *Sindri {
	return &Sindri{
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

func (m *Sindri) Container(ctx context.Context) *dagger.Container {
	return dag.Wolfi().
		Container().
		WithExec([]string{"addgroup", "-S", "-g", gid, group}).
		WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}).
		WithEnvVariable("PATH", home+"/.local/bin:$PATH", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithFile(home+"/.local/bin/sindri", m.Binary(ctx), dagger.ContainerWithFileOpts{Expand: true, Owner: owner}).
		WithExec([]string{"chown", "-R", owner, home}).
		WithUser(user).
		WithEnvVariable("SINDRI_MODULES_DIRECTORY", home+"/.config/sindri/modules", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithDirectory("$SINDRI_MODULES_DIRECTORY", m.Source.Directory("dagger/modules"), dagger.ContainerWithDirectoryOpts{Expand: true, Owner: owner}).
		WithEntrypoint([]string{"sindri"})
}

func (m *Sindri) Service(
	ctx context.Context,
	// +optional
	// +default="localhost"
	hostname string,
) (*dagger.Service, error) {
	ca := dag.TLS().Ca()

	caCrtContents, err := ca.Crt().Contents(ctx)
	if err != nil {
		return nil, err
	}

	keyPair := ca.KeyPair(hostname)
	crtPath := home + "/.config/sindri/tls.crt"
	keyPath := home + "/.config/sindri/tls.key"

	crtContents, err := keyPair.Crt().Contents(ctx)
	if err != nil {
		return nil, err
	}

	return m.Container(ctx).
		WithMountedCache(home+"/.cache/sindri", dag.CacheVolume("sindri")).
		WithFile(keyPath, keyPair.Key(), dagger.ContainerWithFileOpts{Permissions: 0400, Owner: owner}).
		WithFile(crtPath, dag.File(path.Base(crtPath), caCrtContents+crtContents), dagger.ContainerWithFileOpts{Permissions: 0400, Owner: owner}).
		WithExposedPort(5000).
		AsService(dagger.ContainerAsServiceOpts{
			ExperimentalPrivilegedNesting: true,
			UseEntrypoint:                 true,
			Args: []string{
				"--tls-key", keyPath,
				"--tls-crt", crtPath,
			},
		}), nil
}

func (m *Sindri) Generate() (*dagger.Changeset, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{
			"go", "install", "sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0",
		}).
		WithExec([]string{
			"controller-gen", "object", "crd", "webhook", "paths='./internal/...'", "output:crd:artifacts:config=internal/config/crd",
		}).
		Directory(".").
		Changes(m.Source), nil
}

func (m *Sindri) Test(ctx context.Context) (*dagger.Container, error) {
	alias := "sindri.dagger"
	hostname := fmt.Sprintf("%s:5000", alias)
	caCrtPath := "/usr/share/ca-certificates/dagger.crt"

	svc, err := m.Service(ctx, alias)
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
		WithServiceBinding(alias, svc).
		WithExec([]string{"go", "test", "-race", "-cover", "-timeout", "30m", "./e2e/..."}), nil
}

func (m *Sindri) Version(ctx context.Context) string {
	version := "0.0.0-unknown"

	ref, err := m.Source.AsGit().LatestVersion().Ref(ctx)
	if err == nil {
		version = strings.TrimPrefix(ref, "refs/tags/v")
	}

	return version
}

func (m *Sindri) Binary(ctx context.Context) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: []string{".dagger/**", ".github/**", "dagger/modules/**", "e2e/**"},
		}),
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/sindri",
			Ldflags: "-s -w -X main.version=" + m.Version(ctx),
		})
}

func (m *Sindri) Coder() *dagger.LLM {
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
				WithBlockedFunction("Sindri", "binary").
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
