package bucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/frantjc/sindri-module/dagger"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"gocloud.dev/blob"
	"gocloud.dev/blob/azureblob"
	"gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/blob/memblob"
	"gocloud.dev/blob/s3blob"
	"golang.org/x/sync/errgroup"
)

const (
	useSignedURLsParamKey = "use_signed_urls"
)

func init() {
	backend.RegisterBackend(
		backend.BackendOpenerFunc(func(ctx context.Context, u *url.URL) (backend.Backend, error) {
			q := u.Query()

			useSignedURLs := false
			if useSignedURLsParam := q.Get(useSignedURLsParamKey); useSignedURLsParam != "" {
				q.Del(useSignedURLsParamKey)
				useSignedURLs, _ = strconv.ParseBool(q.Get(useSignedURLsParamKey))
			}

			v, _ := url.Parse(u.String())
			v.RawQuery = q.Encode()
			bucket, err := blob.OpenBucket(ctx, v.String())
			if err != nil {
				return nil, err
			}

			s := &Bucket{
				Bucket:        bucket,
				UseSignedURLs: useSignedURLs,
				WorkDir:       os.TempDir(),
			}

			return s, nil
		}),
		azureblob.Scheme,
		fileblob.Scheme,
		gcsblob.Scheme,
		memblob.Scheme,
		s3blob.Scheme,
	)
}

type Bucket struct {
	Bucket        *blob.Bucket
	UseSignedURLs bool
	WorkDir       string
}

// muahahahaha
func beforeWrite(getContentLength func() (int64, error)) func(func(any) bool) error {
	return func(asFunc func(any) bool) error {
		putObjectInput := &s3.PutObjectInput{}

		if asFunc(&putObjectInput) {
			contentLength, err := getContentLength()
			if err != nil {
				return err
			}

			putObjectInput.ContentLength = &contentLength
		}

		return nil
	}
}

// Manifest implements backend.Backend
func (b *Bucket) Store(ctx context.Context, container *dagger.Container, _ *dagger.Client, name, reference string) (digest.Digest, error) {
	tmp := filepath.Join(b.WorkDir, uuid.NewString()+".tar")

	if _, err := container.AsTarball().Export(ctx, tmp); err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	image, err := tarball.ImageFromPath(tmp, nil)
	if err != nil {
		return "", err
	}

	rawManifest, err := image.RawManifest()
	if err != nil {
		return "", err
	}

	d := digest.FromBytes(rawManifest)
	if err = d.Validate(); err != nil {
		return "", err
	}

	eg, egctx := errgroup.WithContext(ctx)
	log := logutil.SloggerFrom(ctx)

	manifest := &v1.Manifest{}
	if err = json.NewDecoder(bytes.NewReader(rawManifest)).Decode(manifest); err != nil {
		return "", err
	}

	eg.Go(func() error {
		key := path.Join("manifests", d.String())

		if ok, err := b.Bucket.Exists(egctx, key); ok {
			return nil
		} else if err != nil {
			return err
		}

		log.Debug("cacheing manifest in bucket", "key", key)

		if err := b.Bucket.WriteAll(egctx, key, rawManifest, &blob.WriterOptions{
			ContentType: string(manifest.MediaType),
			BeforeWrite: beforeWrite(func() (int64, error) {
				return int64(len(rawManifest)), nil
			}),
		}); err != nil {
			return err
		}

		return nil
	})

	eg.Go(func() error {
		key := path.Join("blobs", manifest.Config.Digest.String())

		if ok, err := b.Bucket.Exists(egctx, key); ok {
			return nil
		} else if err != nil {
			return err
		}

		log.Debug("cacheing image config blob in bucket", "key", key)

		rawConfig, err := image.RawConfigFile()
		if err != nil {
			return err
		}

		if err := b.Bucket.WriteAll(egctx, key, rawConfig, &blob.WriterOptions{
			ContentType: string(manifest.Config.MediaType),
			BeforeWrite: beforeWrite(func() (int64, error) {
				return int64(len(rawConfig)), nil
			}),
		}); err != nil {
			return err
		}

		return nil
	})

	layers, err := image.Layers()
	if err != nil {
		return "", err
	}

	for _, layer := range layers {
		eg.Go(func() error {
			hash, err := layer.Digest()
			if err != nil {
				return err
			}

			key := path.Join("blobs", hash.String())

			if ok, err := b.Bucket.Exists(egctx, key); ok {
				return nil
			} else if err != nil {
				return err
			}

			log.Debug("cacheing layer blob in bucket", "key", key)

			rc, err := layer.Compressed()
			if err != nil {
				return err
			}
			defer rc.Close()

			mediaType, err := layer.MediaType()
			if err != nil {
				return err
			}

			if err = b.Bucket.Upload(egctx, key, rc, &blob.WriterOptions{
				ContentType: string(mediaType),
				BeforeWrite: beforeWrite(layer.Size),
			}); err != nil {
				return err
			}

			return nil
		})
	}

	if err = eg.Wait(); err != nil {
		return "", err
	}

	return d, nil
}

// Manifest implements backend.Backend.
func (b *Bucket) Manifest(ctx context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := path.Join("manifests", reference.String())

		attr, err := b.Bucket.Attributes(ctx, key)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		w.Header().Set("Docker-Content-Digest", reference.String())

		if b.UseSignedURLs {
			signedURL, err := b.Bucket.SignedURL(ctx, key, nil)
			if err != nil {
				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}

			http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
			return
		}

		w.Header().Set("Content-Type", attr.ContentType)
		w.Header().Set("Content-Length", fmt.Sprint(attr.Size))

		rc, err := b.Bucket.NewReader(ctx, key, nil)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		defer rc.Close()

		_, _ = io.Copy(w, rc)
	}), nil
}

// Blob implements backend.Backend.
func (b *Bucket) Blob(ctx context.Context, name string, reference digest.Digest) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := path.Join("blobs", reference.String())

		attr, err := b.Bucket.Attributes(ctx, key)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}

		w.Header().Set("Docker-Content-Digest", reference.String())

		if b.UseSignedURLs {
			signedURL, err := b.Bucket.SignedURL(ctx, key, nil)
			if err != nil {
				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}

			http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
			return
		}

		w.Header().Set("Content-Type", attr.ContentType)
		w.Header().Set("Content-Length", fmt.Sprint(attr.Size))

		rc, err := b.Bucket.NewReader(ctx, key, nil)
		if err != nil {
			http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
			return
		}
		defer rc.Close()

		_, _ = io.Copy(w, rc)
	}), nil
}
