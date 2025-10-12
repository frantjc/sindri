// A generated module for Abioticfactor functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"dagger/modules/abioticfactor/internal/dagger"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
)

type Abioticfactor struct{}

const (
	appID = 2857200
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *Abioticfactor) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	rawAppInfo, err := dag.Steamcmd().AppInfoPrint(ctx, appID)
	if err != nil {
		return nil, err
	}

	appInfo := &steamcmd.AppInfo{}

	if err := vdf.NewDecoder(strings.NewReader(rawAppInfo)).Decode(appInfo); err != nil {
		return nil, err
	}

	steamappDirectoryPath := path.Join(home+"/.local/share/sindri/steamapps", fmt.Sprint(appID))

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch:       branch,
		PlatformType: steamcmd.PlatformTypeWindows.String(),
	})

	steamworksSdkRedistLinuxInclude := []string{
		"linux64/**",
		"libsteamwebrtc.so",
		"steamclient.so",
	}

	launch, found := getLaunch(appInfo, isWindows)
	if !found {
		return nil, fmt.Errorf("did not find windows launch config")
	}

	return dag.Layer().
		DirectoryOntoContainer(
			steamappDirectory,
			dag.Debian().
				Container(dagger.DebianContainerOpts{Packages: []string{"winehq-stable"}}).
				WithExec([]string{"groupadd", "-r", "-g", gid, group}).
				WithExec([]string{"useradd", "-m", "-g", group, "-u", uid, "-r", user}),
			steamappDirectoryPath,
			dagger.LayerDirectoryOntoContainerOpts{
				Owner: owner,
				Includes: [][]string{
					steamworksSdkRedistLinuxInclude,
					{"AbioticFactor/Content/**"},
					{"AbioticFactor/Binaries/**"},
				},
				Exclude: []string{
					"steamapps/",
					"steam_appid.txt",
				},
			},
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
