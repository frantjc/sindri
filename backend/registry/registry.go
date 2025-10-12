package registry

import (
	"context"
	"net/url"
	"sync"

	"github.com/frantjc/sindri/backend"
)

type Registry = backend.Backend

type RegistryOpener interface {
	Open(context.Context, *url.URL) (Registry, error)
}

type RegistryOpenerFunc func(context.Context, *url.URL) (Registry, error)

func (s RegistryOpenerFunc) Open(ctx context.Context, u *url.URL) (Registry, error) {
	return s(ctx, u)
}

var (
	registryMux = map[string]RegistryOpener{}
	registryMu  sync.Mutex
)

func RegisterRegistry(opener RegistryOpener, scheme string, schemes ...string) {
	registryMu.Lock()
	defer registryMu.Unlock()

	for _, s := range append(schemes, scheme) {
		if _, overwriting := registryMux[scheme]; overwriting {
			panic("attempt to re-register registry: " + s)
		}

		registryMux[s] = opener
	}
}
