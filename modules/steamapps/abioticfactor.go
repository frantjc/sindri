// A generated module for Abioticfactor functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"dagger/steamapps/internal/dagger"
	"github.com/frantjc/go-steamcmd"
)

type Abioticfactor struct{}

const (
	abioticFactorAppID = 2857200
)

func (m *Abioticfactor) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	steamappDirectory, appInfo, err := appUpdate(ctx, abioticFactorAppID, branch, "", steamcmd.PlatformTypeWindows)
	if err != nil {
		return nil, err
	}

	launch, found := getLaunch(appInfo, isWindows)
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
			{"AbioticFactor/Content/**"},
			{"AbioticFactor/Binaries/**"},
		},
		defaultExclude,
		owner,
		false,
	).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{
			"wine",
			path.Join(steamappDirectoryPath, "Binaries/Win64/AbioticFactorServer-Win64-Shipping.exe"),
			"-useperfthreads",
			"-NoAsyncLoadingThread",
		}).
		WithDefaultArgs(strings.Split(launch.Arguments, " ")), nil
}
