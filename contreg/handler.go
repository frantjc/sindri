package contreg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/frantjc/go-ingress"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/imgutil"
	"github.com/frantjc/sindri/internal/logutil"
	xhttp "github.com/frantjc/x/net/http"
	"github.com/google/uuid"
)

const (
	HeaderDockerContentDigest = "Docker-Content-Digest"
)

func NewPullHandler(puller Puller) http.Handler {
	return ingress.New(
		ingress.ExactPath("/v2/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if puller != nil {
				// OCI does not require this, but the Docker v2 spec include it, and GCR sets this.
				// Docker distribution v2 clients may fallback to an older version if this is not set.
				w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
				w.WriteHeader(http.StatusOK)
				return
			}

			http.NotFound(w, r)
		}), ingress.WithMatchIgnoreSlash),
		ingress.PrefixPath("/v2",
			xhttp.AllowHandler(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if puller == nil {
						http.NotFound(w, r)
						return
					}

					var (
						split    = strings.Split(r.URL.Path, "/")
						lenSplit = len(split)
					)

					if len(split) < 5 {
						http.NotFound(w, r)
						return
					}

					var (
						ep        = split[lenSplit-2]
						name      = strings.Join(split[2:lenSplit-2], "/")
						reference = split[lenSplit-1]
						log       = logutil.SloggerFrom(r.Context()).With(
							"method", r.Method,
							"name", name,
							"reference", reference,
							"request", uuid.NewString(),
						)
					)

					r = r.WithContext(logutil.SloggerInto(r.Context(), log))
					log.Info(ep)

					switch ep {
					case "manifests":
						if r.Method == http.MethodHead {
							if err := puller.HeadManifest(r.Context(), name, reference); err != nil {
								log.Error(ep, "err", err.Error())
								http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
								return
							}

							w.WriteHeader(http.StatusOK)
							return
						}

						manifest, err := puller.GetManifest(r.Context(), name, reference)
						if err != nil {
							log.Error(ep, "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}

						digest, err := imgutil.GetManifestDigest(manifest)
						if err != nil {
							log.Error(ep, "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}

						buf := new(bytes.Buffer)
						if err = json.NewEncoder(buf).Encode(manifest); err != nil {
							log.Error(ep, "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}

						w.Header().Set("Content-Length", fmt.Sprint(buf.Len()))
						w.Header().Set("Content-Type", string(manifest.MediaType))
						w.Header().Set(HeaderDockerContentDigest, digest.String())
						_, _ = io.Copy(w, buf)
						return
					case "blobs":
						if r.Method == http.MethodHead {
							if err := puller.HeadBlob(r.Context(), name, reference); err != nil {
								log.Error(ep, "err", err.Error())
								http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
								return
							}

							w.WriteHeader(http.StatusOK)
							return
						}

						blob, err := puller.GetBlob(r.Context(), name, reference)
						if err != nil {
							log.Error(ep, "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}

						hash, err := blob.Digest()
						if err != nil {
							log.Error("blob digest", "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}

						w.Header().Set(HeaderDockerContentDigest, hash.String())

						rc, err := blob.Compressed()
						if err != nil {
							log.Error("compressed blob reader", "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}
						defer rc.Close()

						_, _ = io.Copy(w, rc)
						return
					default:
						http.NotFound(w, r)
						return
					}
				}),
				[]string{http.MethodGet, http.MethodHead},
			),
		),
	)
}
