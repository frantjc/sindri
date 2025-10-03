// A generated module for Corekeeper functions

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
	"github.com/frantjc/sindri/dagger/modules/corekeeper/internal/dagger"
)

type Corekeeper struct{}

const (
	appID = 1963720
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
		WithExec([]string{"groupadd", "-r", group}).
		WithExec([]string{"useradd", "-m", "-g", group, "-r", user}).
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
