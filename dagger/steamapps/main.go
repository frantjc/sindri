// A generated module for Steamapps functions

package main

import (
	"dagger/steamapps/internal/dagger"
)

type Steamapps struct{}

func (m *Steamapps) Container(name, reference string) *dagger.Container {
	switch name {
	case "abioticfactor", "2857200":
		return dag.Abioticfactor().
			Container(dagger.AbioticfactorContainerOpts{
				Branch: reference,
			})
	}

	// TODO(frantjc): LLM?
	return dag.Container()
}
