// The interface that a Dagger module must implement to work with Sindri.

// Defines the standard interface for Sindri-compatible Dagger modules.
//
// The <name> parameter represents the full image name being requested, and the
// <reference> parameter specifies the tag or reference to build. Modules should
// parse the name to determine how to build the appropriate container.

package main

import (
	"dagger/interface/internal/dagger"
)

type Sindri struct{}

func (m *Sindri) Container(name, reference string) *dagger.Container {
	return dag.Container()
}
