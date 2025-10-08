package sindri

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/frantjc/go-ingress"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	xhttp "github.com/frantjc/x/net/http"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"gocloud.dev/blob"
	"golang.org/x/sync/errgroup"
)

type PullRegistry struct {
	ImageBuilder  ImageBuilder
	Bucket        *blob.Bucket
	UseSignedURLs bool
}

const (
	defaultBranchName = "public"
)

func (p *PullRegistry) headManifest(ctx context.Context, name string, reference string) error {
	log := logutil.SloggerFrom(ctx)

	if reference == "latest" {
		// Special handling for mapping the default image tag to the default Steamapp branch name.
		reference = defaultBranchName
	}

	if err := digest.Digest(reference).Validate(); err == nil {
		key := path.Join("manifests", reference)

		log.Debug("checking bucket for digest reference", "key", key)

		rc, err := p.Bucket.NewReader(ctx, key, nil)
		if err != nil {
			return err
		}
		defer rc.Close()

		manifest := &v1.Manifest{}

		if err = jsonDecoderStrict(rc).Decode(manifest); err != nil {
			return err
		}
	}

	return nil
}

func jsonDecoderStrict(r io.Reader) *json.Decoder {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	return d
}

func (p *PullRegistry) getManifest(ctx context.Context, name string, reference string) ([]byte, digest.Digest, string, error) {
	log := logutil.SloggerFrom(ctx)

	if reference == "latest" {
		// Special handling for mapping the default image tag to the default Steamapp branch name.
		reference = defaultBranchName
	} else if dig := digest.Digest(reference); dig.Validate() == nil {
		// If the reference is a digest instead of a Steamapp branch name, it necessarily
		// must have been generated previously to be retrievable.
		key := path.Join("manifests", reference)

		log.Debug("checking bucket for digest reference", "key", key)

		rc, err := p.Bucket.NewReader(ctx, key, nil)
		if err != nil {
			return nil, "", "", err
		}
		defer rc.Close()

		var (
			manifest = &v1.Manifest{}
			buf      = new(bytes.Buffer)
		)

		if err = jsonDecoderStrict(io.TeeReader(rc, buf)).Decode(manifest); err != nil {
			return nil, "", "", err
		}

		return buf.Bytes(), dig, string(manifest.MediaType), nil
	}

	opener, err := p.ImageBuilder.BuildImage(ctx, name, reference)
	if err != nil {
		return nil, "", "", err
	}
	defer opener.Close()

	image, err := tarball.Image(opener.Open, nil)
	if err != nil {
		return nil, "", "", err
	}

	rawManifest, err := image.RawManifest()
	if err != nil {
		return nil, "", "", err
	}

	dig := digest.FromBytes(rawManifest)
	if err = dig.Validate(); err != nil {
		return nil, "", "", err
	}

	eg, egctx := errgroup.WithContext(ctx)

	manifest, err := image.Manifest()
	if err != nil {
		return nil, "", "", err
	}

	eg.Go(func() error {
		key := path.Join("manifests", dig.String())

		log.Debug("cacheing manifest in bucket", "key", key)

		wc, err := p.Bucket.NewWriter(egctx, key, &blob.WriterOptions{
			ContentType: string(manifest.MediaType),
		})
		if err != nil {
			return err
		}
		defer wc.Close()

		if _, err = wc.Write(rawManifest); err != nil {
			return err
		}

		return wc.Close()
	})

	eg.Go(func() error {
		key := path.Join("blobs", manifest.Config.Digest.String())

		if ok, err := p.Bucket.Exists(egctx, key); ok {
			return nil
		} else if err != nil {
			return err
		}

		log.Debug("cacheing image config blob in bucket", "key", key)

		cfgfb, err := image.RawConfigFile()
		if err != nil {
			return err
		}

		wc, err := p.Bucket.NewWriter(egctx, key, &blob.WriterOptions{
			ContentType: string(manifest.Config.MediaType),
		})
		if err != nil {
			return err
		}
		defer wc.Close()

		if _, err := wc.Write(cfgfb); err != nil {
			return err
		}

		return wc.Close()
	})

	layers, err := image.Layers()
	if err != nil {
		return nil, "", "", err
	}

	for _, layer := range layers {
		eg.Go(func() error {
			hash, err := layer.Digest()
			if err != nil {
				return err
			}

			key := path.Join("blobs", hash.String())

			if ok, err := p.Bucket.Exists(egctx, key); ok {
				return nil
			} else if err != nil {
				return err
			}

			log.Debug("cacheing layer blob in bucket", "key", key, "digest", hash.String())

			rc, err := layer.Compressed()
			if err != nil {
				return err
			}
			defer rc.Close()

			mediaType, err := layer.MediaType()
			if err != nil {
				return err
			}

			if err = p.Bucket.Upload(egctx, key, rc, &blob.WriterOptions{
				ContentType:     string(mediaType),
			}); err != nil {
				return err
			}

			return rc.Close()
		})
	}

	if err = eg.Wait(); err != nil {
		return nil, "", "", err
	}

	return rawManifest, dig, string(manifest.MediaType), nil
}

func (p *PullRegistry) headBlob(ctx context.Context, digest string) error {
	log := logutil.SloggerFrom(ctx)

	hash, err := v1.NewHash(digest)
	if err != nil {
		return err
	}

	key := path.Join("blobs", hash.String())

	log.Debug("checking bucket for digest reference", "key", key)

	if ok, err := p.Bucket.Exists(ctx, key); ok {
		return nil
	} else if err != nil {
		return err
	}

	return fmt.Errorf("blob not found: %s", digest)
}

func (p *PullRegistry) getBlob(ctx context.Context, digest string) (io.ReadCloser, string, string, error) {
	log := logutil.SloggerFrom(ctx)

	hash, err := v1.NewHash(digest)
	if err != nil {
		return nil, "", "", err
	}

	key := path.Join("blobs", hash.String())

	log.Debug("checking bucket for digest reference", "key", key)

	attr, err := p.Bucket.Attributes(ctx, key)
	if err != nil {
		return nil, "", "", err
	}

	if p.UseSignedURLs {
		signedURL, err := p.Bucket.SignedURL(ctx, key, nil)
		if err != nil {
			return nil, "", "", err
		}

		return nil, "", signedURL, nil
	}

	rc, err := p.Bucket.NewReader(ctx, key, nil)
	if err != nil {
		return nil, "", "", err
	}

	return rc, attr.ContentType, "", nil
}

const (
	headerDockerContentDigest = "Docker-Content-Digest"
)

func (p *PullRegistry) Handler() http.Handler {
	return ingress.New(
		ingress.ExactPath("/v2/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// OCI does not require this, but the Docker v2 spec include it, and GCR sets this.
			// Docker distribution v2 clients may fallback to an older version if this is not set.
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		}), ingress.WithMatchIgnoreSlash),
		ingress.PrefixPath("/v2",
			xhttp.AllowHandler(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
							if err := p.headManifest(r.Context(), name, reference); err != nil {
								log.Error(ep, "err", err.Error())
								http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
								return
							}

							w.WriteHeader(http.StatusOK)
							return
						}

						rawManifest, dig, mediaType, err := p.getManifest(r.Context(), name, reference)
						if err != nil {
							log.Error(ep, "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}

						w.Header().Set("Content-Length", fmt.Sprint(len(rawManifest)))
						w.Header().Set("Content-Type", mediaType)
						w.Header().Set(headerDockerContentDigest, dig.String())
						_, _ = w.Write(rawManifest)
						return
					case "blobs":
						if r.Method == http.MethodHead {
							if err := p.headBlob(r.Context(), reference); err != nil {
								log.Error(ep, "err", err.Error())
								http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
								return
							}

							w.WriteHeader(http.StatusOK)
							return
						}

						blob, mediaType, signedURL, err := p.getBlob(r.Context(), reference)
						if err != nil {
							log.Error(ep, "err", err.Error())
							http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
							return
						}
						w.Header().Set(headerDockerContentDigest, reference)
						if signedURL != "" {
							log.Debug("redirecting", "signed-url", signedURL)
							http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
							return
						}
						defer blob.Close()

						w.Header().Set("Content-Type", mediaType)

						_, _ = io.Copy(w, blob)
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
