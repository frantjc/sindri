// A generated module for Satisfactory functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"dagger/modules/satisfactory/internal/dagger"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
)

type Satisfactory struct{}

const (
	appID = 1690800
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *Satisfactory) Container(
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

	steamappDirectoryPath := home+"/.local/share/sindri/steamapp"

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch: branch,
	})

	steamworksSdkRedistLinuxInclude := []string{
		"linux64/**",
		"libsteamwebrtc.so",
		"steamclient.so",
	}

	launch, found := getLaunch(appInfo, isLinux)
	if !found {
		return nil, fmt.Errorf("did not find linux launch config")
	}

	return dag.Layer().
		DirectoryOntoContainer(
			steamappDirectory,
			dag.Wolfi().
				Container().
				WithExec([]string{"addgroup", "-S", "-g", gid, group}).
				WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}),
			steamappDirectoryPath,
			dagger.LayerDirectoryOntoContainerOpts{
				Owner: owner,
				Includes: [][]string{
					steamworksSdkRedistLinuxInclude,
					{"FactoryGame/**"},
					{"Engine/Plugins/**"},
					{"Engine/Binaries/**"},
				},
				Exclude: []string{
					"steamapps/",
					"steam_appid.txt",
				},
			},
		).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{path.Join(steamappDirectoryPath, launch.Executable)}), nil
}

var (
	isLinux = supportsOS("linux")
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
