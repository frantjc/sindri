// A generated module for Corekeeper functions

package main

import (
	"context"
	"fmt"
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

	steamappDirectoryPath := path.Join("/opt/sindri/steamapps", fmt.Sprint(appID))

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch: branch,
	})

	steamClientSoPath := path.Join(steamappDirectoryPath, "linux64/steamclient.so")

	steamClientSoLinkPath := path.Join("/usr/lib/x86_64-linux-gnu", path.Base(steamClientSoPath))

	return dag.Debian().
		Container(dagger.DebianContainerOpts{
			Packages: []string{
				"ca-certificates",
				"curl",
				"locales",
				"libxi6",
				"xvfb",
			},
		}).
		WithExec([]string{
			"ln", "-s",
			steamClientSoPath,
			steamClientSoLinkPath,
		}).
		WithExec([]string{"groupadd", "-r", "-g", gid, group}).
		WithExec([]string{"useradd", "-m", "-g", group, "-u", uid, "-r", user}).
		WithDirectory(
			steamappDirectoryPath,
			steamappDirectory,
			dagger.ContainerWithDirectoryOpts{Owner: owner},
		).
		WithUser(user).
		WithEntrypoint([]string{
			path.Join(steamappDirectoryPath, "_launch.sh"),
			"-logfile", "/dev/stdout",
		}), nil
}
