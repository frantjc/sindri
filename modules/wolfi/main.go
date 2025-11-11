// A Sindri module that builds Wolfi-based containers with the given packages.

// Enables building Docker containers using the Wolfi Linux distribution
// with specified packages pre-installed.
//
// The `name` parameter should be formatted as a slash-separated list of
// package names to install, and the `reference` parameter is ignored.
//
// For example, `docker pull localhost:5000/curl/jq/git` will build
// a Wolfi container with curl, jq, and git packages installed.
//
// All valid Wolfi package names are supported. Invalid package names may
// cause build failures during container creation.

package main

import (
	"dagger/wolfi/internal/dagger"
	"slices"
	"strings"
)

type Sindri struct{}

func (m *Sindri) Container(name, reference string) *dagger.Container {
	packages := strings.Split(name, "/")
	slices.Sort(packages)
	return dag.Wolfi().Container(dagger.WolfiContainerOpts{
		Packages: packages,
	})
}
