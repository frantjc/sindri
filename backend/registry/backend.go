package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/pkg/cmd/factory"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/frantjc/sindri-module/dagger"
	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	xslices "github.com/frantjc/x/slices"
	specs "github.com/opencontainers/distribution-spec/specs-go/v1"
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
				if repository == "" {
					return nil, fmt.Errorf("repository cannot be empty for %s: try %s://%s/<user>", host, Scheme, host)
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

func (r *Registry) getRegistryAuth(ctx context.Context, ref string) (string, string, bool, error) {
	if r.Username != "" && r.Password != "" {
		return r.Username, r.Password, true, nil
	}

	switch {
	case r.Host == "ghcr.io":
		cfg, err := factory.New("v0.0.0-unknown").Config()
		if err != nil {
			return "", "", false, err
		}

		authCfg := cfg.Authentication()

		httpClient, err := api.NewHTTPClient(api.HTTPClientOptions{
			Config: authCfg,
		})
		if err != nil {
			return "", "", false, err
		}

		username, err := authCfg.ActiveUser("github.com")
		if err != nil {
			var nerr error
			username, nerr = api.CurrentLoginName(api.NewClientFromHTTP(httpClient), "github.com")
			if nerr != nil {
				return "", "", false, fmt.Errorf("%v: %v", err, nerr)
			}
		}

		password, _ := authCfg.ActiveToken("github.com")

		return username, password, true, nil
	case xslices.Some([]string{".azurecr.io", ".azurecr.us", ".azurecr.cn"}, func(suffix string, _ int) bool {
		return strings.HasSuffix(r.Host, suffix)
	}):
		return r.getRegistryAuthForProvider(ctx, ref, azure.ProviderName)
	case strings.HasSuffix(r.Host, ".amazonaws.com"):
		return r.getRegistryAuthForProvider(ctx, ref, aws.ProviderName)
	}

	return "", "", false, nil
}

func (r *Registry) getRegistryAuthForProvider(ctx context.Context, ref, provider string) (string, string, bool, error) {
	authOpts := []auth.Option{}
	if provider == azure.ProviderName {
		authOpts = append(authOpts, auth.WithAllowShellOut())
	}

	authenticator, err := authutils.GetArtifactRegistryCredentials(ctx, provider, fmt.Sprintf("oci://%s", ref), authOpts...)
	if err != nil {
		return "", "", false, err
	}

	authConfig, err := authenticator.Authorization()
	if err != nil {
		return "", "", false, err
	}

	return authConfig.Username, authConfig.Password, true, nil
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
func (r *Registry) Store(ctx context.Context, container *dagger.Container, dag *dagger.Client, name, reference string) (digest.Digest, error) {
	ref := fmt.Sprintf("%s:%s",
		path.Join(r.Host, r.Repository, name),
		reference,
	)

	username, password, ok, err := r.getRegistryAuth(ctx, ref)
	if err != nil {
		return "", err
	} else if ok {
		container = container.WithRegistryAuth(r.Host, username, dag.SetSecret("password", password))
	}

	address, err := container.Publish(ctx, ref)
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
	return b.proxy("", "/v2", b.Repository, name, "manifests", reference.String()), nil
}

// Blob implements backend.Backend.
func (b *Registry) Blob(_ context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return b.proxy("", "/v2", b.Repository, name, "blobs", reference.String()), nil
}

// Close implements backend.Backend.
func (b *Registry) Close() error {
	return nil
}

// Root implements backend.AuthBackend.
func (b *Registry) Root(context.Context) (http.Handler, error) {
	return b.proxy("", "/v2/"), nil
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

		var body io.Reader = res.Body
		if res.StatusCode >= 400 {
			buf := new(bytes.Buffer)
			body = io.TeeReader(body, buf)
			go func() {
				errors := specs.ErrorResponse{}
				if err := json.NewDecoder(buf).Decode(&errors); err == nil {
					for _, e := range errors.Errors {
						args := []any{}
						if e.Code != "" {
							args = append(args, "code", e.Code)
						}
						if e.Detail != "" {
							args = append(args, "detail", e.Detail)
						}
						log.Error(e.Message, args...)
					}
				} else {
					log.Error(buf.String())
				}
			}()
		}

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
		_, _ = io.Copy(w, body)
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
