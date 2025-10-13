// A generated module for Sindri functions

package main

import (
	"dagger/wolfi/internal/dagger"
	"strings"
)

type Sindri struct{}

func (m *Sindri) Container(name, reference string) *dagger.Container {
	return dag.Wolfi().Container(dagger.WolfiContainerOpts{
		Packages: strings.Split(name, "/"),
	})
}
