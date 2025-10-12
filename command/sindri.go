package command

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/adrg/xdg"
	"github.com/frantjc/sindri"
	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/frantjc/steamapps/dagger"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	cache = path.Join(xdg.CacheHome, "sindri")
)

func NewSindri() *cobra.Command {
	var (
		address  string
		storage  string
		certFile string
		keyFile  string
		cmd      = &cobra.Command{
			Use: "sindri",
			RunE: func(cmd *cobra.Command, _ []string) error {
				var (
					eg, ctx = errgroup.WithContext(cmd.Context())
					srv     = &http.Server{
						ReadHeaderTimeout: time.Second * 5,
						BaseContext: func(_ net.Listener) context.Context {
							return cmd.Context()
						},
						ErrorLog: log.New(io.Discard, "", 0),
					}
					log = logutil.SloggerFrom(ctx)
				)

				lis, err := net.Listen("tcp", address)
				if err != nil {
					return err
				}
				defer lis.Close()

				c, err := dagger.Connect(ctx)
				if err != nil {
					return err
				}
				defer c.Close()

				b, err := backend.OpenBackend(ctx, storage)
				if err != nil {
					return err
				}
				// TODO(frantjc): defer b.Close()?

				srv.Handler = sindri.Handler(c, b)

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

	cmd.Flags().StringVar(&address, "addr", ":5000", "Address to listen on")
	cmd.Flags().StringVar(&storage, "backend", fmt.Sprintf("file://%s?create_dir=1&no_tmp_dir=1", cache), "Storage backend URL")

	cmd.Flags().StringVar(&certFile, "tls-crt", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "tls-key", "", "TLS private key file")
	cmd.MarkFlagsRequiredTogether("tls-crt", "tls-key")

	return cmd
}
