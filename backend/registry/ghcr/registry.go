package ghcr

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/backend/registry"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/frantjc/sindri-module/dagger"
	"github.com/opencontainers/go-digest"
)

const (
	Host = "ghcr.io"
)

func init() {
	registry.RegisterRegistry(
		registry.RegistryOpenerFunc(func(ctx context.Context, u *url.URL) (registry.Registry, error) {
			password, _ := u.User.Password()
			repository := strings.TrimPrefix(u.Path, "/")

			if strings.Count(repository, "/") < 1 {
				return nil, fmt.Errorf("path must be of the format org/repo")
			}

			return &Registry{
				Repository: repository,
				Username:   cmp.Or(u.User.Username(), os.Getenv("GITHUB_ACTOR")),
				Password:   cmp.Or(password, os.Getenv("GITHUB_TOKEN")),
			}, nil
		}),
		Host,
		"ghcr",
	)
}

type Registry struct {
	Repository string
	Username   string
	Password   string
}

var (
	_ backend.AuthBackend = new(Registry)
)

// Store implements backend.Backend.
func (r *Registry) Store(ctx context.Context, container *dagger.Container, client *dagger.Client, name, reference string) (digest.Digest, error) {
	address, err := container.
		WithRegistryAuth(Host,
			r.Username,
			client.SetSecret("github-token", r.Password),
		).
		Publish(ctx,
			fmt.Sprintf("%s:%s",
				path.Join(Host, r.Repository, name),
				reference,
			),
		)
	if err != nil {
		return "", err
	}

	_, d, found := strings.Cut(address, "@")
	if !found {
		return "", fmt.Errorf("parse digest from %s", address)
	}

	return digest.Digest(d), nil
}

func (b *Registry) proxy(name, api string, reference digest.Digest) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(fmt.Sprintf("https://%s/v2", Host))
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, u.JoinPath(b.Repository, name, api, reference.String()).String(), nil)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		req.Header = r.Header.Clone()

		w.Header().Set("X-Redirected", req.URL.String())

		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		defer res.Body.Close()

		for k, v := range res.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		// Hopefully this is a redirect so we don't have to proxy massive blobs.
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}), nil
}

// Manifest implements backend.Backend.
func (b *Registry) Manifest(_ context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return b.proxy(name, "manifests", reference)
}

// Blob implements backend.Backend.
func (b *Registry) Blob(_ context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return b.proxy(name, "blobs", reference)
}

// Root implements backend.AuthBackend.
func (r *Registry) Root(context.Context) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logutil.SloggerFrom(r.Context())

		req, err := http.NewRequestWithContext(r.Context(), r.Method, fmt.Sprintf("https://%s/v2/", Host), nil)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		w.Header().Set("X-Redirected", req.URL.String())

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		defer res.Body.Close()

		for k, v := range res.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		if wwwAuth := res.Header.Get("Www-Authenticate"); wwwAuth != "" {
			scheme := "http"
			if xForwardedProto := r.Header.Get("X-Forwarded-Proto"); xForwardedProto != "" {
				scheme = xForwardedProto
			} else if r.TLS != nil {
				scheme = "https"
			}

			rewrittenWwwAuth := strings.Replace(wwwAuth, fmt.Sprintf("https://%s/", Host), fmt.Sprintf("%s://%s/v2/", scheme, r.Host), 1)
			log.Debug("Www-Authenticate", "before", wwwAuth, "after", rewrittenWwwAuth)
			w.Header().Set("Www-Authenticate", rewrittenWwwAuth)
		}

		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}), nil
}

// Token implements backend.AuthBackend.
func (b *Registry) Token(context.Context) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logutil.SloggerFrom(r.Context())
		q := r.URL.Query()
		// NB: The incoming scope is "repository:<name>:pull". We specifically do not
		// append <name> to b.Repository here in case b.Repository/<name> doesn't exist,
		// which causes GitHub to 403 us. Surprisingly, this works.
		scope := "repository:" + b.Repository + ":pull"
		log.Debug("scope", "before", q.Get("scope"), "after", scope)
		q.Set("scope", scope)

		req, err := http.NewRequestWithContext(r.Context(), r.Method, fmt.Sprintf("https://%s/token?%s", Host, q.Encode()), nil)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		w.Header().Set("X-Redirected", req.URL.String())

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		defer res.Body.Close()

		for k, v := range res.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}), nil
}
