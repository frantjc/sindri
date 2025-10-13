// A generated module for Sindri functions

package main

import (
	"dagger/steamapps/internal/dagger"
)

type Sindri struct{}

func (m *Sindri) Container(name, reference string) *dagger.Container {
	if reference == "latest" {
		reference = "public"
	}

	switch name {
	case "abioticfactor":
		return dag.Abioticfactor().
			Container(dagger.AbioticfactorContainerOpts{
				Branch: reference,
			})
	case "astroneer":
		return dag.Astroneer().
			Container(dagger.AstroneerContainerOpts{
				Branch: reference,
			})
	case "corekeeper":
		return dag.Corekeeper().
			Container(dagger.CorekeeperContainerOpts{
				Branch: reference,
			})
	case "enshrouded":
		return dag.Enshrouded().
			Container(dagger.EnshroudedContainerOpts{
				Branch: reference,
			})
	case "palworld":
		return dag.Palworld().
			Container(dagger.PalworldContainerOpts{
				Branch: reference,
			})
	case "satisfactory":
		return dag.Satisfactory().
			Container(dagger.SatisfactoryContainerOpts{
				Branch: reference,
			})
	case "valheim":
		return dag.Valheim().
			Container(dagger.ValheimContainerOpts{
				Branch: reference,
			})
	}

	// TODO(frantjc): LLM?
	return dag.Container()
}
