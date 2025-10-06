// A generated module for Valheim functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
	"github.com/frantjc/sindri/dagger/modules/valheim/internal/dagger"
)

type Valheim struct{}

const (
	appID = 896660
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
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

	steamappDirectoryPath := path.Join("/opt/sindri/steamapps", fmt.Sprint(appID))

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch: branch,
	})

	steamClientSoPath := path.Join(steamappDirectoryPath, "linux64/steamclient.so")

	steamClientSoLinkPath := path.Join("/usr/lib", path.Base(steamClientSoPath))

	return dag.Wolfi().
		Container(dagger.WolfiContainerOpts{
			Packages: []string{"zlib"},
		}).
		WithExec([]string{
			"ln", "-s",
			steamClientSoPath,
			steamClientSoLinkPath,
		}).
		WithExec([]string{"addgroup", "-S", "-g", gid, group}).
		WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}).
		WithDirectory(
			steamappDirectoryPath,
			steamappDirectory,
			dagger.ContainerWithDirectoryOpts{Owner: owner},
		).
		WithExec([]string{
			"rm", "-r",
			path.Join(steamappDirectoryPath, "docker"),
			path.Join(steamappDirectoryPath, "docker_start_server.sh"),
			path.Join(steamappDirectoryPath, "start_server_xterm.sh"),
			path.Join(steamappDirectoryPath, "start_server.sh"),
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
