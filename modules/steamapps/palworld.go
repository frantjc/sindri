// A generated module for Palworld functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"dagger/steamapps/internal/dagger"
	"github.com/frantjc/go-steamcmd"
)

type Palworld struct{}

const (
	palworldAppID = 2394010
)

func (m *Palworld) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	steamappDirectory, appInfo, err := appUpdate(ctx, palworldAppID, branch, "", steamcmd.PlatformTypeLinux)
	if err != nil {
		return nil, err
	}

	launch, found := getLaunch(appInfo, isLinux)
	if !found {
		return nil, fmt.Errorf("did not find linux launch config")
	}

	return layerDirectoryOntoContainer(
		ctx,
		steamappDirectory,
		dag.Wolfi().
			Container(dagger.WolfiContainerOpts{
				Packages: []string{"ca-certificates-bundle"},
			}).
			WithExec([]string{"addgroup", "-S", "-g", gid, group}).
			WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}),
		steamappDirectoryPath,
		[][]string{
			steamworksSdkRedistLinuxInclude,
			{"Pal/Content/**"},
		},
		defaultExclude,
		owner,
		false,
	).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{path.Join(steamappDirectoryPath, launch.Executable)}).
		WithDefaultArgs(strings.Split(launch.Arguments, " ")), nil
}
