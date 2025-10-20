// A Sindri module that builds Steamapp dedicated server containers.

// Enables building Docker containers for various Steamapp dedicated servers.
//
// The `name` parameter should match a supported game name (e.g. "palworld",
// "valheim", "satisfactory"), and the `reference` parameter specifies the
// server branch, with "latest" defaulting to "public".
//
// For example, `docker pull localhost:5000/valheim:publictest` will build
// a Valheim Dedicated Server container using the publictest branch.
//
// Supported games include Abiotic Factor, Astroneer, Core Keeper, Enshrouded,
// Palworld, Satisfactory, and Valheim. Unsupported names return an empty
// container which will cause an error when exporting or publishing.

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
