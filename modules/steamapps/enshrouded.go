// A generated module for Enshrouded functions

package main

import (
	"context"
	"fmt"
	"path"

	"dagger/steamapps/internal/dagger"
	"github.com/frantjc/go-steamcmd"
)

type Enshrouded struct{}

const (
	enshroudedAppID = 2278520
)

func (m *Enshrouded) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	steamappDirectory, appInfo, err := appUpdate(ctx, enshroudedAppID, branch, "", steamcmd.PlatformTypeWindows)
	if err != nil {
		return nil, err
	}

	launch, found := getLaunch(appInfo, func(launch *steamcmd.AppInfoConfigLaunch) bool {
		return true
	})
	if !found {
		return nil, fmt.Errorf("did not find windows launch config")
	}

	return layerDirectoryOntoContainer(
		ctx,
		steamappDirectory,
		debian("winehq-stable").
			WithExec([]string{"groupadd", "-r", "-g", gid, group}).
			WithExec([]string{"useradd", "-m", "-g", group, "-u", uid, "-r", user}),
		steamappDirectoryPath,
		[][]string{
			steamworksSdkRedistLinuxInclude,
			{"enshrouded_server.kfc"},
			{"enshrouded_server_00*.dat"},
			{"enshrouded_server_01*.dat"},
			{"enshrouded_server_02*.dat"},
			{"enshrouded_server_03*.dat"},
		},
		defaultExclude,
		owner, false,
	).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{
			"wine",
			path.Join(steamappDirectoryPath, launch.Executable),
		}), nil
}
