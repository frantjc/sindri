// A generated module for Abioticfactor functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
	"github.com/frantjc/sindri/dagger/modules/abioticfactor/internal/dagger"
)

type Abioticfactor struct{}

const (
	appID = 2857200
	group = "sindri"
	user  = group
	owner = user + ":" + group
)

func (m *Abioticfactor) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch,
	// +optional
	betaPassword string,
) (*dagger.Container, error) {
	rawAppInfo, err := dag.Steamcmd().AppInfoPrint(ctx, appID)
	if err != nil {
		return nil, err
	}

	appInfo := &steamcmd.AppInfo{}

	if err := vdf.NewDecoder(strings.NewReader(rawAppInfo)).Decode(appInfo); err != nil {
		return nil, err
	}

	steamappDirectoryPath := path.Join("/opt/sindri/steamapps", fmt.Sprint(appID))

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch:       branch,
		BetaPassword: betaPassword,
		PlatformType: steamcmd.PlatformTypeWindows.String(),
	})

	launch, found := getLaunch(appInfo, isWindows)
	if !found {
		return nil, fmt.Errorf("did not find windows launch config")
	}

	return dag.Debian().
		Container(dagger.DebianContainerOpts{Packages: []string{"winehq-stable"}}).
		WithExec([]string{"groupadd", "-r", group}).
		WithExec([]string{"useradd", "-m", "-g", group, "-r", user}).
		WithDirectory(
			steamappDirectoryPath,
			steamappDirectory,
			dagger.ContainerWithDirectoryOpts{Owner: owner},
		).
		WithUser(user).
		WithEntrypoint([]string{
			"wine",
			path.Join(steamappDirectoryPath, "Binaries/Win64/AbioticFactorServer-Win64-Shipping.exe"),
			"-useperfthreads",
			"-NoAsyncLoadingThread",
		}).
		WithDefaultArgs(strings.Split(launch.Arguments, " ")), nil
}

var (
	isWindows = supportsOS("windows")
)

func supportsOS(os string) func(launch *steamcmd.AppInfoConfigLaunch) bool {
	return func(launch *steamcmd.AppInfoConfigLaunch) bool {
		return strings.Contains(launch.Config.OSList, os)
	}
}

func getLaunch(appInfo *steamcmd.AppInfo, f func(launch *steamcmd.AppInfoConfigLaunch) bool) (*steamcmd.AppInfoConfigLaunch, bool) {
	for _, launch := range appInfo.Config.Launch {
		if f(&launch) {
			return &launch, true
		}
	}

	return nil, false
}

