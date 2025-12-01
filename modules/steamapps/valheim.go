// A generated module for Valheim functions

package main

import (
	"context"
	"path"

	"dagger/steamapps/internal/dagger"
	"github.com/frantjc/go-steamcmd"
)

type Valheim struct{}

const (
	valheimAppID = 896660
)

func (m *Valheim) Container(
	ctx context.Context,
	// +optional
	// +default="public"
	branch string,
) (*dagger.Container, error) {
	steamappDirectory, _, err := appUpdate(ctx, valheimAppID, branch, "", steamcmd.PlatformTypeLinux)
	if err != nil {
		return nil, err
	}

	return layerDirectoryOntoContainer(
		ctx,
		steamappDirectory,
		dag.Wolfi().
			Container(dagger.WolfiContainerOpts{
				Packages: []string{"zlib"},
			}).
			WithExec([]string{"addgroup", "-S", "-g", gid, group}).
			WithExec([]string{"adduser", "-S", "-G", group, "-u", uid, user}),
		steamappDirectoryPath,
		[][]string{
			steamworksSdkRedistLinuxInclude,
			{"valheim_server_Data/StreamingAssets/**"},
		},
		append(defaultExclude,
			"docker/",
			"docker_start_server.sh",
			"start_server_xterm.sh",
			"start_server.sh",
		),
		owner, false,
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
