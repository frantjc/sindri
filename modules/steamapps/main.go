// A generated module for Sindri functions

package main

import (
	"context"
	"dagger/steamapps/internal/dagger"
	"fmt"
	"maps"
	"strings"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
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

func (m *Sindri) Tags(ctx context.Context, name string) ([]string, error) {
	var appID int

	switch name {
	case "abioticfactor":
		appID = 2857200
	case "astroneer":
		appID = 728470
	case "corekeeper":
		appID = 1963720
	case "enshrouded":
		appID = 2278520
	case "palworld":
		appID = 2394010
	case "satisfactory":
		appID = 1690800
	case "valheim":
		appID = 896660
	default:
		return nil, fmt.Errorf("unknown name %s", name)
	}

	rawAppInfo, err := dag.Steamcmd().AppInfoPrint(ctx, appID)
	if err != nil {
		return nil, err
	}

	appInfo := &steamcmd.AppInfo{}

	if err := vdf.NewDecoder(strings.NewReader(rawAppInfo)).Decode(appInfo); err != nil {
		return nil, err
	}

	branches := make([]string, len(appInfo.Depots.Branches))
	i := 0

	for branch := range maps.Keys(appInfo.Depots.Branches) {
		branches[i] = branch
		i++
	}

	return branches, nil
}
