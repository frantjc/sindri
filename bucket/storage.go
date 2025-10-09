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
	"github.com/frantjc/sindri"
	"github.com/frantjc/sindri/internal/logutil"
	dagger "github.com/frantjc/steamapps/client"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
	"golang.org/x/sync/errgroup"
)

func init() {
	sindri.RegisterStorage(
		sindri.StorageOpenerFunc(func(ctx context.Context, u *url.URL) (sindri.Storage, error) {
			useSignedURLs, _ := strconv.ParseBool(u.Query().Get("use-signed-urls"))

			bucket, err := blob.OpenBucket(ctx, u.String())
			if err != nil {
				return nil, err
			}

			s := &storage{
				Bucket:        bucket,
				UseSignedURLs: useSignedURLs,
				WorkDir: os.TempDir(),
			}

			return s, nil
		}),
		s3blob.Scheme,
	)
}

type storage struct {
	Bucket        *blob.Bucket
	UseSignedURLs bool
	WorkDir string
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

// Store implements sindri.Storage.
func (s *storage) Store(ctx context.Context, container *dagger.Container) (sindri.Responder, error) {
	tmp := filepath.Join(s.WorkDir, uuid.NewString()+".tar")

	if _, err := container.AsTarball().Export(ctx, tmp); err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	image, err := tarball.Image(func() (io.ReadCloser, error) { return os.Open(tmp) }, nil)
	if err != nil {
		return nil, err
	}

	rawManifest, err := image.RawManifest()
	if err != nil {
		return nil, err
	}

	dig := digest.FromBytes(rawManifest)
	if err = dig.Validate(); err != nil {
		return nil, err
	}

	eg, egctx := errgroup.WithContext(ctx)
	log := logutil.SloggerFrom(ctx)

	manifest := &v1.Manifest{}
	if err = json.NewDecoder(bytes.NewReader(rawManifest)).Decode(manifest); err != nil {
		return nil, err
	}

	eg.Go(func() error {
		key := path.Join("manifests", dig.String())

		if ok, err := s.Bucket.Exists(egctx, key); ok {
			return nil
		} else if err != nil {
			return err
		}

		log.Debug("cacheing manifest in bucket", "key", key)

		if err := s.Bucket.WriteAll(egctx, key, rawManifest, &blob.WriterOptions{
			ContentType: string(manifest.Config.MediaType),
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

		if ok, err := s.Bucket.Exists(egctx, key); ok {
			return nil
		} else if err != nil {
			return err
		}

		log.Debug("cacheing image config blob in bucket", "key", key)

		rawConfig, err := image.RawConfigFile()
		if err != nil {
			return err
		}

		if err := s.Bucket.WriteAll(egctx, key, rawConfig, &blob.WriterOptions{
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
		return nil, err
	}

	for _, layer := range layers {
		eg.Go(func() error {
			hash, err := layer.Digest()
			if err != nil {
				return err
			}

			key := path.Join("blobs", hash.String())

			if ok, err := s.Bucket.Exists(egctx, key); ok {
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

			if err = s.Bucket.Upload(egctx, key, rc, &blob.WriterOptions{
				ContentType: string(mediaType),
				BeforeWrite: beforeWrite(layer.Size),
			}); err != nil {
				return err
			}

			return nil
		})
	}

	if err = eg.Wait(); err != nil {
		return nil, err
	}

	return sindri.ResponderFunc(func(w http.ResponseWriter) error {
		w.Header().Set("Content-Length", fmt.Sprint(len(rawManifest)))
		w.Header().Set("Content-Type", string(manifest.MediaType))
		w.Header().Set("Docker-Content-Digest", dig.String())

		if _, err := w.Write(rawManifest); err != nil {
			return err
		}

		return nil
	}), nil
}
