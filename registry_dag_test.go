//go:build dagger

package sindri_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/frantjc/sindri/internal/dagger"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

func WithProgress(t testing.TB) remote.Option {
	ctx := t.Context()
	updates := make(chan v1.Update)
	w := t.Output()
	go func() {
		for {
			select {
			case <-ctx.Done():
				require.ErrorIs(t, ctx.Err(), context.Canceled)
				return
			case update := <-updates:
				require.NoError(t, update.Error)
				_, err := fmt.Fprintf(w, "%d/%d", update.Complete, update.Total)
				require.NoError(t, err)
			}
		}
	}()
	return remote.WithProgress(updates)
}

func Dag(t testing.TB) *dagger.Client {
	ctx := t.Context()
	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})
	return dag
}

func Registry(t testing.TB, dag *dagger.Client, module string) string {
	ctx := t.Context()
	// FIXME(frantjc): Hopefuly a temporary workaround for dag.SindriDev() not being generated.
	svc, err := new(dagger.SindriDev).
		WithGraphQLQuery(dag.QueryBuilder().Select("sindriDev")).
		Service(dagger.SindriDevServiceOpts{
			Module: module,
		}).
		Start(ctx)
	require.NoError(t, err)
	require.NotNil(t, svc)
	t.Cleanup(func() {
		ctx := context.WithoutCancel(ctx)
		_, err = svc.Stop(ctx)
		require.NoError(t, err)
	})
	ep, err := svc.Endpoint(ctx)
	require.NoError(t, err)
	return ep
}
