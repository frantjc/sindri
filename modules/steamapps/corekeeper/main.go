// A generated module for Corekeeper functions

package main

import (
	"context"
	"path"
	"strings"

	"dagger/modules/corekeeper/internal/dagger"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
)

type Corekeeper struct{}

const (
	appID = 1963720
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *Corekeeper) Container(
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
		Branch: branch,
	})

	steamClientSoPath := path.Join(steamappDirectoryPath, "linux64/steamclient.so")

	steamClientSoLinkPath := path.Join("/usr/lib/x86_64-linux-gnu", path.Base(steamClientSoPath))

	steamworksSdkRedistLinuxInclude := []string{
		"linux64/**",
		"libsteamwebrtc.so",
		"steamclient.so",
	}

	return dag.Layer().
		DirectoryOntoContainer(
			steamappDirectory,
			dag.Debian().
				Container(dagger.DebianContainerOpts{
					Packages: []string{
						"ca-certificates",
						"curl",
						"locales",
						"libxi6",
						"xvfb",
					},
				}).
				WithExec([]string{"groupadd", "-r", "-g", gid, group}).
				WithExec([]string{"useradd", "-m", "-g", group, "-u", uid, "-r", user}),
			steamappDirectoryPath,
			dagger.LayerDirectoryOntoContainerOpts{
				Owner: owner,
				Includes: [][]string{
					steamworksSdkRedistLinuxInclude,
					{"CoreKeeperServer_Data/Managed/**"},
					{"CoreKeeperServer_Data/Plugins/**"},
					{"CoreKeeperServer_Data/StreamingAssets/**"},
				},
				Exclude: []string{
					"steamapps/",
					"steam_appid.txt",
					"_readme.sh",
					"launch.sh",
				},
			},
		).
		WithExec([]string{
			"ln", "-s",
			steamClientSoPath,
			steamClientSoLinkPath,
		}).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{
			path.Join(steamappDirectoryPath, "_launch.sh"),
			"-logfile", "/dev/stdout",
		}), nil
}
