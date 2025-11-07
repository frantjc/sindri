package registry

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/frantjc/sindri-module/dagger"
	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/opencontainers/go-digest"
)

const Scheme = "registry"

const (
	tokenPathParamKey = "token_path"
	tlsVerifyParamKey = "tls_verify"
)

func init() {
	backend.RegisterBackend(
		backend.BackendOpenerFunc(func(ctx context.Context, u *url.URL) (backend.Backend, error) {
			var (
				scheme      = "https"
				username    = u.User.Username()
				password, _ = u.User.Password()
				host        = u.Host
				repository  = strings.TrimPrefix(u.Path, "/")
				tokenPath   = "/token"
			)

			if tlsVerify, err := strconv.ParseBool(u.Query().Get(tlsVerifyParamKey)); err == nil && !tlsVerify {
				scheme = "http"
			}

			switch host {
			case "ghcr.io":
				username = cmp.Or(username, os.Getenv("GITHUB_ACTOR"))
				password = cmp.Or(password, os.Getenv("GITHUB_TOKEN"))

				if repository == "" {
					placeholder := "<user>"
					if username != "" {
						placeholder = username
					}

					return nil, fmt.Errorf("repository cannot be empty for %s: try %s://%s/%s", host, Scheme, host, placeholder)
				}
			case "gcr.io":
				tokenPath = "/v2/token"
			}

			if paramTokenPath := u.Query().Get(tokenPathParamKey); paramTokenPath != "" {
				tokenPath = paramTokenPath
			}

			return &Registry{
				Scheme:     scheme,
				Host:       host,
				Username:   username,
				Password:   password,
				Repository: repository,
				TokenPath:  tokenPath,
			}, nil
		}),
		Scheme,
	)
}

type Registry struct {
	Scheme     string
	Username   string
	Password   string
	Host       string
	Repository string
	TokenPath  string
}

var (
	_ backend.AuthBackend = new(Registry)
)

// Store implements backend.Backend.
func (r *Registry) Store(ctx context.Context, container *dagger.Container, client *dagger.Client, name, reference string) (digest.Digest, error) {
	address, err := container.
		WithRegistryAuth(r.Host,
			r.Username,
			client.SetSecret("github-token", r.Password),
		).
		Publish(ctx,
			fmt.Sprintf("%s:%s",
				path.Join(r.Host, r.Repository, name),
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

// Manifest implements backend.Backend.
func (b *Registry) Manifest(_ context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return b.proxy("", "/v2", name, "manifests", reference.String()), nil
}

// Blob implements backend.Backend.
func (b *Registry) Blob(_ context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return b.proxy("", "/v2", name, "blobs", reference.String()), nil
}

// Root implements backend.AuthBackend.
func (b *Registry) Root(context.Context) (http.Handler, error) {
	return b.proxy("", "/v2"), nil
}

// Token implements backend.AuthBackend.
func (b *Registry) Token(context.Context) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logutil.SloggerFrom(r.Context())
		q := r.URL.Query()
		// NB: The incoming scope is "repository:<name>:pull". We specifically do not
		// append <name> to b.Repository here in case b.Repository/<name> doesn't already exist,
		// which causes ghcr to 403 the before we get to create b.Repository/<name> for the user.
		// Surprisingly, this works.
		scope := "repository:" + b.Repository + ":pull"
		log.Debug("scope", "before", q.Get("scope"), "after", scope)
		q.Set("scope", scope)
		b.proxy(q.Encode(), b.TokenPath).ServeHTTP(w, r)
	}), nil
}

func (b *Registry) proxy(query string, elem ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logutil.SloggerFrom(r.Context())

		u := b.getURL(elem...)
		u.RawQuery = query

		req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), nil)
		if err != nil {
			log.Error(err.Error())
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		req.Header = r.Header.Clone()

		w.Header().Set("X-Redirected", req.URL.String())

		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			log.Error(err.Error())
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

			if r != nil {
				if xForwardedProto := r.Header.Get("X-Forwarded-Proto"); xForwardedProto != "" {
					scheme = xForwardedProto
				} else if r.TLS != nil {
					scheme = "https"
				}
			}

			rewrittenWwwAuth := strings.Replace(wwwAuth, b.getTokenURL().String(), fmt.Sprintf("%s://%s/v2/token", scheme, r.Host), 1)
			log.Debug("Www-Authenticate", "before", wwwAuth, "after", rewrittenWwwAuth)
			w.Header().Set("Www-Authenticate", rewrittenWwwAuth)
		}

		// Hopefully this is a redirect so we don't have to proxy massive blobs.
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	})
}

func (b *Registry) getURL(elem ...string) *url.URL {
	return (&url.URL{
		Scheme: b.Scheme,
		Host:   b.Host,
	}).JoinPath(elem...)
}

func (b *Registry) getTokenURL() *url.URL {
	return b.getURL(b.TokenPath)
}
