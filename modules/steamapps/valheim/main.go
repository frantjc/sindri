// A generated module for Valheim functions

package main

import (
	"context"
	"path"
	"strings"

	"dagger/modules/valheim/internal/dagger"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
)

type Valheim struct{}

const (
	appID = 896660
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user
)

func (m *Valheim) Container(
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

	steamClientSoPath := path.Join(steamappDirectoryPath, "linux64/steamclient.so")

	steamClientSoLinkPath := path.Join("/usr/lib", path.Base(steamClientSoPath))

	steamworksSdkRedistLinuxInclude := []string{
		"linux64/**",
		"libsteamwebrtc.so",
		"steamclient.so",
	}

	return dag.Layer().
		DirectoryOntoContainer(
			steamappDirectory,
			dag.Wolfi().
				Container(dagger.WolfiContainerOpts{
					Packages: []string{"zlib"},
				}).
				WithExec([]string{"addgroup", "-S", "-g", gid, group}).
				WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}),
			steamappDirectoryPath,
			dagger.LayerDirectoryOntoContainerOpts{
				Owner: owner,
				Includes: [][]string{
					steamworksSdkRedistLinuxInclude,
					{"valheim_server_Data/StreamingAssets/**"},
				},
				Exclude: []string{
					"docker/",
					"steamapps/",
					"steam_appid.txt",
					"docker_start_server.sh",
					"start_server_xterm.sh",
					"start_server.sh",
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
		WithEntrypoint([]string{path.Join(steamappDirectoryPath, "valheim_server.x86_64")}).
		WithDefaultArgs([]string{
			"-name", "My server",
			"-port", "2456",
			"-world", "Dedicated",
			"-password", "secret",
			"-crossplay",
		}), nil
}
