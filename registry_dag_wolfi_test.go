//go:build dagger && wolfi

package sindri_test

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

func TestModuleWolfiPullOnePackage(t *testing.T) {
	ctx := t.Context()
	dag := Dag(t)
	registry := Registry(t, dag, "wolfi")
	ref, err := name.ParseReference(fmt.Sprintf("%s/curl", registry), name.Insecure)
	require.NoError(t, err)
	img, err := remote.Image(ref, remote.WithContext(ctx), WithProgress(t))
	require.NoError(t, err)
	_, err = img.Digest()
	require.NoError(t, err)
}

func TestModuleWolfiPullMultiplePackage(t *testing.T) {
	ctx := t.Context()
	dag := Dag(t)
	registry := Registry(t, dag, "wolfi")
	ref, err := name.ParseReference(fmt.Sprintf("%s/curl/wget", registry), name.Insecure)
	require.NoError(t, err)
	img, err := remote.Image(ref, remote.WithContext(ctx), WithProgress(t))
	require.NoError(t, err)
	_, err = img.Digest()
	require.NoError(t, err)
}
