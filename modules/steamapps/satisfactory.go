// A generated module for Satisfactory functions

package main

import (
	"context"
	"fmt"
	"path"

	"dagger/steamapps/internal/dagger"
	"github.com/frantjc/go-steamcmd"
)

type Satisfactory struct{}

const (
	satisfactoryAppID = 1690800
)

func (m *Satisfactory) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	steamappDirectory, appInfo, err := appUpdate(ctx, satisfactoryAppID, branch, "", steamcmd.PlatformTypeLinux)
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
			Container().
			WithExec([]string{"addgroup", "-S", "-g", gid, group}).
			WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}),
		steamappDirectoryPath,
		[][]string{
			steamworksSdkRedistLinuxInclude,
			{"FactoryGame/**"},
			{"Engine/Plugins/**"},
			{"Engine/Binaries/**"},
		},
		defaultExclude,
		owner,
		false,
	).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{path.Join(steamappDirectoryPath, launch.Executable)}), nil
}
