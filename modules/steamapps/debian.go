// A generated module for Debian functions

package main

import (
	"path"
	"slices"

	"dagger/steamapps/internal/dagger"
	xslices "github.com/frantjc/x/slices"
)

func debian(packages ...string) *dagger.Container {
	container := dag.Container().
		From("debian:stable-slim")

	if len(packages) > 0 {
		if xslices.Some(packages, func(aptPackage string, _ int) bool {
			return slices.Contains([]string{"winehq-stable", "winehq-devel", "winehq-staging"}, aptPackage)
		}) {
			container = withWinehq(debian("ca-certificates"))
		}

		container = container.
			WithExec([]string{"apt-get", "update", "-y"}).
			WithExec(append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, packages...)).
			WithExec([]string{"apt-get", "clean"}).
			WithExec([]string{"rm", "-rf", "/var/lib/apt/lists/*"})
	}

	return container
}

const (
	winehqKeyURL         = "https://dl.winehq.org/wine-builds/winehq.key"
	winehqArchiveKeyPath = "/etc/apt/keyrings/winehq-archive.key"
)

func withWinehq(container *dagger.Container) *dagger.Container {
	return withWinehqSources(withWinehqKey(container))
}

func withWinehqSources(container *dagger.Container) *dagger.Container {
	return container.
		WithExec([]string{
			"dpkg", "--add-architecture", "i386",
		}).
		WithFile(
			"/tmp/winehq.sources",
			debian("ca-certificates", "curl").
				WithExec([]string{
					"bash", "-c",
					`. /etc/os-release && curl -o "/tmp/winehq.sources" "https://dl.winehq.org/wine-builds/$ID/dists/${VERSION_CODENAME}/winehq-${VERSION_CODENAME}.sources"`,
				}).
				File("/tmp/winehq.sources"),
		).
		WithExec([]string{
			"bash", "-c",
			`. /etc/os-release && mv "/tmp/winehq.sources" "/etc/apt/sources.list.d/winehq-${VERSION_CODENAME}.sources"`,
		})
}

func withWinehqKey(container *dagger.Container) *dagger.Container {
	var (
		rawWinehqKeyPath       = path.Join("/tmp", path.Base(winehqKeyURL))
		dearmoredWinehqKeyPath = path.Join("/tmp", path.Base(winehqArchiveKeyPath))
	)

	return container.WithFile(
		winehqArchiveKeyPath,
		debian("gpg").
			WithFile(
				rawWinehqKeyPath,
				dag.HTTP(winehqKeyURL),
			).
			WithExec([]string{
				"gpg", "--dearmor",
				"--output", dearmoredWinehqKeyPath,
				rawWinehqKeyPath,
			}).
			File(dearmoredWinehqKeyPath),
	)
}
