package sindri

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	dagger "github.com/frantjc/steamapps/client"
)

type Responder interface {
	Respond(http.ResponseWriter) error
}

type ResponderFunc func(http.ResponseWriter) error

func (r ResponderFunc) Respond(w http.ResponseWriter) error {
	return r(w)
}

type Storage interface {
	Store(context.Context, *dagger.Container) (Responder, error)
}

type StorageOpener interface {
	Open(context.Context, *url.URL) (Storage, error)
}

type StorageOpenerFunc func(context.Context, *url.URL) (Storage, error)

func (s StorageOpenerFunc) Open(ctx context.Context, u *url.URL) (Storage, error) {
	return s(ctx, u)
}

var (
	storageMux = map[string]StorageOpener{}
	storageMu sync.Mutex
)

func RegisterStorage(opener StorageOpener, scheme string, schemes... string) {
	storageMu.Lock()
	defer storageMu.Unlock()

	for _, s := range append(schemes, scheme) {
		if _, overwriting := storageMux[scheme]; overwriting {
			panic("attempt to re-register storage scheme: " + s)
		}

		storageMux[s] = opener
	}
}
