package registry

import (
	"context"
	"fmt"
	"net/url"

	"github.com/frantjc/sindri/backend"
)

func init() {
	backend.RegisterBackend(
		backend.BackendOpenerFunc(func(ctx context.Context, u *url.URL) (backend.Backend, error) {
			registryOpener, ok := registryMux[u.Host]
			if !ok {
				return nil, fmt.Errorf("unknown registry %s", u.Host)
			}

			registry, err := registryOpener.Open(ctx, u)
			if err != nil {
				return nil, err
			}

			return registry, nil
		}),
		"registry",
	)
}
