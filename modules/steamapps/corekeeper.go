// A generated module for Corekeeper functions

package main

import (
	"context"
	"path"

	"dagger/steamapps/internal/dagger"
	"github.com/frantjc/go-steamcmd"
)

type Corekeeper struct{}

const (
	coreKeeperAppID = 1963720
)

func (m *Corekeeper) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	steamappDirectory, _, err := appUpdate(ctx, coreKeeperAppID, branch, "", steamcmd.PlatformTypeLinux)
	if err != nil {
		return nil, err
	}

	return layerDirectoryOntoContainer(
		ctx,
		steamappDirectory,
		debian("ca-certificates",
			"curl",
			"locales",
			"libxi6",
			"xvfb",
		).
			WithExec([]string{"groupadd", "-r", "-g", gid, group}).
			WithExec([]string{"useradd", "-m", "-g", group, "-u", uid, "-r", user}),
		steamappDirectoryPath,
		[][]string{
			steamworksSdkRedistLinuxInclude,
			{"CoreKeeperServer_Data/Managed/**"},
			{"CoreKeeperServer_Data/Plugins/**"},
			{"CoreKeeperServer_Data/StreamingAssets/**"},
		},
		append(defaultExclude, "_readme.sh", "launch.sh"),
		owner, false,
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
