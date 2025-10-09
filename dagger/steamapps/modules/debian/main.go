// A generated module for Debian functions

package main

import (
	"path"
	"slices"

	"dagger/modules/debian/internal/dagger"
	xslices "github.com/frantjc/x/slices"
)

type Debian struct {
	// +private
	From string
}

func New(
	// +optional
	// +default="debian:stable-slim"
	from string,
) *Debian {
	return &Debian{From: from}
}

func (m *Debian) Container(
	// +optional
	packages ...string,
) *dagger.Container {
	container := dag.Container().
		From("debian:stable-slim")

	if len(packages) > 0 {
		if xslices.Some(packages, func(aptPackage string, _ int) bool {
			return slices.Contains([]string{"winehq-stable", "winehq-devel", "winehq-staging"}, aptPackage)
		}) {
			container = m.withWinehq(m.Container("ca-certificates"))
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

func (m *Debian) withWinehq(container *dagger.Container) *dagger.Container {
	return m.withWinehqSources(m.withWinehqKey(container))
}

func (m *Debian) withWinehqSources(container *dagger.Container) *dagger.Container {
	return container.
		WithExec([]string{
			"dpkg", "--add-architecture", "i386",
		}).
		WithFile(
			"/tmp/winehq.sources",
			m.Container("ca-certificates", "curl").
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

func (m *Debian) withWinehqKey(container *dagger.Container) *dagger.Container {
	var (
		rawWinehqKeyPath       = path.Join("/tmp", path.Base(winehqKeyURL))
		dearmoredWinehqKeyPath = path.Join("/tmp", path.Base(winehqArchiveKeyPath))
	)

	return container.WithFile(
		winehqArchiveKeyPath,
		m.Container("gpg").
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
