package command

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/adrg/xdg"
	"github.com/frantjc/sindri"
	"github.com/frantjc/sindri-module/dagger"
	"github.com/frantjc/sindri/backend"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	cache = path.Join(xdg.CacheHome, "sindri")
)

func NewSindri(version string) *cobra.Command {
	var (
		address    string
		storage    string
		certFile   string
		keyFile    string
		slogConfig = new(logutil.SlogConfig)
		cmd        = &cobra.Command{
			Use:           "sindri",
			Version:       version,
			SilenceErrors: true,
			SilenceUsage:  true,
			PersistentPreRun: func(cmd *cobra.Command, _ []string) {
				handler := slog.NewTextHandler(cmd.OutOrStdout(), &slog.HandlerOptions{
					Level: slogConfig,
				})
				cmd.SetContext(logutil.SloggerInto(cmd.Context(), slog.New(handler)))
			},
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

	cmd.Flags().BoolP("help", "h", false, "Help for "+cmd.Name())
	cmd.Flags().Bool("version", false, "Version for "+cmd.Name())
	cmd.SetVersionTemplate("{{ .Name }}{{ .Version }}")

	slogConfig.AddFlags(cmd.Flags())

	cmd.Flags().StringVar(&address, "addr", ":5000", "Address to listen on")
	cmd.Flags().StringVar(&storage, "backend", fmt.Sprintf("file://%s?create_dir=1&no_tmp_dir=1", cache), "Storage backend URL")

	cmd.Flags().StringVar(&certFile, "tls-crt", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "tls-key", "", "TLS private key file")
	cmd.MarkFlagsRequiredTogether("tls-crt", "tls-key")

	return cmd
}
