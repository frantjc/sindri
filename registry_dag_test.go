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
	go func() {
		for {
			select {
			case <-ctx.Done():
				require.ErrorIs(t, ctx.Err(), context.Canceled)
				return
			case update := <-updates:
				require.NoError(t, update.Error)
				fmt.Fprintf(t.Output(), "%d/%d", update.Complete, update.Total)
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
	// FIXME(frantjc): Hopefuly a temporary workaround for dag.Sindri() not being generated.
	svc, err := new(dagger.SindriDev).
		WithGraphQLQuery(dag.QueryBuilder().Select("sindriDev")).
		Service(dagger.SindriDevServiceOpts{
			Module: module,
		}).
		Start(ctx)
	t.Cleanup(func() {
		ctx := context.WithoutCancel(ctx)
		_, err = svc.Stop(ctx)
		require.NoError(t, err)
	})
	require.NoError(t, err)
	ep, err := svc.Endpoint(ctx)
	require.NoError(t, err)
	return ep
}
