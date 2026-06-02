//go:build dagger && steamapps

package sindri_test

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

func TestModuleSteamappsPullCorekeeper(t *testing.T) {
	ctx := t.Context()
	dag := Dag(t)
	registry := Registry(t, dag, "steamapps")
	ref, err := name.ParseReference(fmt.Sprintf("%s/corekeeper", registry), name.Insecure)
	require.NoError(t, err)
	img, err := remote.Image(ref, remote.WithContext(ctx), WithProgress(t))
	require.NoError(t, err)
	_, err = img.Digest()
	require.NoError(t, err)
}

func TestModuleSteamappsPullSatisfactory(t *testing.T) {
	if true {
		t.Skip("skipping to avoid GitHub Actions disk size limitations")
	}
	ctx := t.Context()
	dag := Dag(t)
	registry := Registry(t, dag, "steamapps")
	ref, err := name.ParseReference(fmt.Sprintf("%s/satisfactory", registry), name.Insecure)
	require.NoError(t, err)
	img, err := remote.Image(ref, remote.WithContext(ctx), WithProgress(t))
	require.NoError(t, err)
	_, err = img.Digest()
	require.NoError(t, err)
}

func TestModuleSteamappsPullValheim(t *testing.T) {
	if true {
		t.Skip("skipping to avoid GitHub Actions disk size limitations")
	}
	ctx := t.Context()
	dag := Dag(t)
	registry := Registry(t, dag, "steamapps")
	ref, err := name.ParseReference(fmt.Sprintf("%s/valheim", registry), name.Insecure)
	require.NoError(t, err)
	img, err := remote.Image(ref, remote.WithContext(ctx), WithProgress(t))
	require.NoError(t, err)
	_, err = img.Digest()
	require.NoError(t, err)
}
