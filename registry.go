package sindri

import (
	"net/http"
	"strings"

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

func Handler(dag *dagger.Client, b backend.Backend) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v2", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/v2/", http.StatusMovedPermanently)
	})

	// TODO(frantjc): use github.com/opencontainers/distribution-spec/specs-go/v1.ErrorResponse with correct error codes
	// instead of http.Error(). See https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes.
	if ab, ok := b.(backend.AuthBackend); ok {
		mux.HandleFunc("GET /v2/{$}", func(w http.ResponseWriter, r *http.Request) {
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
		mux.HandleFunc("GET /v2/{$}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		})
	}

	mux.HandleFunc("GET /v2/{pathname...}", func(w http.ResponseWriter, r *http.Request) {
		pathname := r.PathValue("pathname")
		parts := strings.Split(pathname, "/")
		lenParts := len(parts)
		if lenParts < 3 {
			http.NotFound(w, r)
			return
		}

		apiIndex := lenParts - 2
		api := parts[apiIndex]
		name := strings.Join(parts[:apiIndex], "/")
		reference := parts[lenParts-1]
		ctx := r.Context()
		log := logutil.SloggerFrom(ctx).With("name", name, "reference", reference)
		ctx = logutil.SloggerInto(ctx, log)

		switch api {
		case "manifests":
			d, ok := dig(reference)
			if !ok {
				var err error
				if d, err = b.Store(
					r.Context(),
					dag.Sindri().
						Container(name, reference),
					dag,
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
		case "blobs":
			handler, err := b.Blob(
				r.Context(),
				name, digest.Digest(reference),
			)
			if err != nil {
				log.Error(err.Error())
				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}

			handler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logutil.SloggerFrom(ctx).With("request", uuid.NewString())
		log.Info(r.Method + " " + r.URL.Path)
		mux.ServeHTTP(w, r.WithContext(logutil.SloggerInto(ctx, log)))
	})
}
