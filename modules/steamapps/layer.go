// A generated module for Layer functions

package main

import (
	"context"

	"dagger/steamapps/internal/dagger"
	xslices "github.com/frantjc/x/slices"
)

func layerDirectoryOntoContainer(
	ctx context.Context,
	directory *dagger.Directory,
	container *dagger.Container,
	path string,
	// +optional
	includes [][]string,
	// +optional
	exclude []string,
	// +optional
	owner string,
	// +optional
	expand bool,
) *dagger.Container {
	for _, include := range includes {
		container = container.WithDirectory(path, directory, dagger.ContainerWithDirectoryOpts{
			Include: include,
			Owner: owner,
			Expand: expand,
			Exclude: exclude,
		})
	}
	
	return container.WithDirectory(path, directory, dagger.ContainerWithDirectoryOpts{
		Owner: owner,
		Expand: expand,
		Exclude: append(exclude, xslices.Flatten(includes...)...),
	})
}
