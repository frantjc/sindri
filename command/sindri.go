package command

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/frantjc/sindri/internal/cache"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/frantjc/sindri"
	"github.com/frantjc/sindri/dagger"
	"github.com/spf13/cobra"
	"gocloud.dev/blob"
	"golang.org/x/sync/errgroup"
)

func NewSindri() *cobra.Command {
	var (
		port         int
		bucket       string
		certFile     string
		keyFile      string
		imageBuilder = new(dagger.ImageBuilder)
		cmd          = &cobra.Command{
			Use: "sindri",
			RunE: func(cmd *cobra.Command, _ []string) error {
				var (
					eg, ctx  = errgroup.WithContext(cmd.Context())
					log      = logutil.SloggerFrom(ctx)
					registry = &sindri.PullRegistry{ImageBuilder: imageBuilder}
					srv      = &http.Server{
						ReadHeaderTimeout: time.Second * 5,
						Handler:           registry.Handler(),
						BaseContext: func(_ net.Listener) context.Context {
							return cmd.Context()
						},
					}
				)

				lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
				if err != nil {
					return err
				}
				defer lis.Close()

				registry.Bucket, err = blob.OpenBucket(ctx, bucket)
				if err != nil {
					return err
				}

				eg.Go(func() error {
					<-ctx.Done()
					if err = srv.Shutdown(context.WithoutCancel(ctx)); err != nil {
						return err
					}
					return ctx.Err()
				})

				eg.Go(func() error {
					log.Info("listening...", "addr", lis.Addr().String())

					if certFile != "" {
						return srv.ServeTLS(lis, certFile, keyFile)
					}

					return srv.Serve(lis)
				})

				return eg.Wait()
			},
		}
	)

	cmd.Flags().IntVar(&port, "port", 5000, "Port to listen on")
	cmd.Flags().StringVar(&bucket, "bucket", fmt.Sprintf("file://%s?create_dir=1&no_tmp_dir=1", filepath.Join(cache.Dir, "registry")), "Bucket URL")

	cmd.Flags().StringVar(&certFile, "tls-crt", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "tls-key", "", "TLS private key file")
	cmd.MarkFlagsRequiredTogether("tls-crt", "tls-key")

	cmd.Flags().StringVar(&imageBuilder.ModulesDirectory, "modules-directory", os.Getenv("SINDRI_MODULES_DIRECTORY"), "Path to Sindri's Dagger modules")
	cmd.Flags().StringVar(&imageBuilder.ModulesRef, "modules-git-ref", os.Getenv("SINDRI_MODULES_GIT_REF"), "Git ref of Sindri's Dagger modules")

	return cmd
}
