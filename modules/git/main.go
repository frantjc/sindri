// A Sindri module that `docker build`s a directory of a given repository.

// Enables the building Docker containers from Git repositories,
//
// The `name` parameter should be formatted "host/owner/repo[/subdirectory/path]",
// and the `reference` parameter specifies the Git reference to use, with "latest"
// defaulting to HEAD.
//
// For example, `docker pull localhost:5000/github.com/frantjc/sindri/testdata:main`
// will build the Dockerfile in the testdata/ subdirectory of the main branch of this
// repository.
//
// If the name format is invalid (i.e. fewer than 3 path segments), it returns
// an empty container which will cause an error when exporing or publishing it.

package main

import (
	"dagger/git/internal/dagger"
	"fmt"
	"strings"
)

type Sindri struct{}

func (m *Sindri) Container(name, reference string) *dagger.Container {
	parts := strings.Split(name, "/")

	if len(parts) > 2 {
		gitRepo := dag.Git(fmt.Sprintf("https://%s", strings.Join(parts[:3], "/")))
		gitRef := gitRepo.Head()

		if reference != "latest" {
			gitRef = gitRepo.Ref(reference)
		}

		dir := gitRef.Tree()
		if len(parts) > 3 {
			dir = dir.Directory(strings.Join(parts[3:], "/"))
		}

		return dir.DockerBuild()
	}

	return dag.Container()
}
