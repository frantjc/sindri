package sindri

import (
	"net/http"

	"github.com/frantjc/sindri-module/dagger"
	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
)

func dig(reference string) (digest.Digest, bool) {
	d := digest.Digest(reference)
	return d, d.Validate() == nil
}

func Handler(c *dagger.Client, b backend.Backend) http.Handler {
	mux := http.NewServeMux()

	if ab, ok := b.(backend.AuthBackend); ok {
		mux.HandleFunc("GET /v2/", func(w http.ResponseWriter, r *http.Request) {
			log := logutil.SloggerFrom(r.Context())

			handler, err := ab.Root(r.Context())
			if err != nil {
				log.Error(err.Error())
				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}

			handler.ServeHTTP(w, r)
		})

		mux.HandleFunc("GET /v2/token", func(w http.ResponseWriter, r *http.Request) {
			log := logutil.SloggerFrom(r.Context())

			handler, err := ab.Token(r.Context())
			if err != nil {
				log.Error(err.Error())
				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}

			handler.ServeHTTP(w, r)
		})
	} else {
		mux.HandleFunc("GET /v2/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		})
	}

	mux.HandleFunc("GET /v2/{name}/manifests/{reference}", func(w http.ResponseWriter, r *http.Request) {
		log := logutil.SloggerFrom(r.Context())

		name := r.PathValue("name")
		reference := r.PathValue("reference")

		d, ok := dig(reference)
		if !ok {
			var err error
			if d, err = b.Store(
				r.Context(),
				c.Sindri().
					Container(name, reference),
				c,
				name,
				reference,
			); err != nil {
				log.Error(err.Error())
				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}
		}

		handler, err := b.Manifest(
			r.Context(),
			name, d,
		)
		if err != nil {
			log.Error(err.Error())
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		handler.ServeHTTP(w, r)
	})

	mux.HandleFunc("GET /v2/{name}/blobs/{reference}", func(w http.ResponseWriter, r *http.Request) {
		log := logutil.SloggerFrom(r.Context())

		name := r.PathValue("name")
		d := digest.Digest(r.PathValue("reference"))

		handler, err := b.Blob(
			r.Context(),
			name, d,
		)
		if err != nil {
			log.Error(err.Error())
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		handler.ServeHTTP(w, r)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logutil.SloggerFrom(ctx).With("request", uuid.NewString())
		log.Info(r.Method + " " + r.URL.Path)
		mux.ServeHTTP(w, r.WithContext(logutil.SloggerInto(ctx, log)))
	})
}
