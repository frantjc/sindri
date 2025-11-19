// A generated module for Astroneer functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"dagger/modules/astroneer/internal/dagger"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
)

type Astroneer struct{}

const (
	appID = 728470
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *Astroneer) Container(
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

	steamappDirectoryPath := home + "/.local/share/sindri/steamapp"

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch:       branch,
		PlatformType: steamcmd.PlatformTypeWindows.String(),
	})

	steamworksSdkRedistLinuxInclude := []string{
		"linux64/**",
		"libsteamwebrtc.so",
		"steamclient.so",
	}

	launch, found := getLaunch(appInfo, func(launch *steamcmd.AppInfoConfigLaunch) bool {
		return true
	})
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
					{"Astro/Content/**"},
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
			path.Join(steamappDirectoryPath, launch.Executable),
		}), nil
}

func getLaunch(appInfo *steamcmd.AppInfo, f func(launch *steamcmd.AppInfoConfigLaunch) bool) (*steamcmd.AppInfoConfigLaunch, bool) {
	for _, launch := range appInfo.Config.Launch {
		if f(&launch) {
			return &launch, true
		}
	}

	return nil, false
}
