package backend

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/frantjc/sindri-module/dagger"
	"github.com/opencontainers/go-digest"
)

type Backend interface {
	Store(context.Context, *dagger.Container, *dagger.Client, string, string) (digest.Digest, error)
	Manifest(context.Context, string, digest.Digest) (http.Handler, error)
	Blob(context.Context, string, digest.Digest) (http.Handler, error)
}

type AuthBackend interface {
	Backend
	Root(context.Context) (http.Handler, error)
	Token(context.Context) (http.Handler, error)
}

type BackendOpener interface {
	Open(context.Context, *url.URL) (Backend, error)
}

type BackendOpenerFunc func(context.Context, *url.URL) (Backend, error)

func (s BackendOpenerFunc) Open(ctx context.Context, u *url.URL) (Backend, error) {
	return s(ctx, u)
}

var (
	backendMux = map[string]BackendOpener{}
	backendMu  sync.Mutex
)

func RegisterBackend(opener BackendOpener, scheme string, schemes ...string) {
	backendMu.Lock()
	defer backendMu.Unlock()

	for _, s := range append(schemes, scheme) {
		if _, overwriting := backendMux[scheme]; overwriting {
			panic("attempt to re-register backend scheme: " + s)
		}

		backendMux[s] = opener
	}
}

func OpenBackend(ctx context.Context, urlstr string) (Backend, error) {
	backendMu.Lock()
	defer backendMu.Unlock()

	u, err := url.Parse(urlstr)
	if err != nil {
		return nil, err
	}

	backendOpener, ok := backendMux[u.Scheme]
	if !ok {
		return nil, fmt.Errorf("unknown backend %s", u.Scheme)
	}

	return backendOpener.Open(ctx, u)
}
