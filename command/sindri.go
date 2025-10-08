package command

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/adrg/xdg"
	"github.com/frantjc/sindri"
	"github.com/frantjc/sindri/dagger"
	"github.com/frantjc/sindri/internal/api"
	"github.com/frantjc/sindri/internal/controller"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/spf13/cobra"
	"gocloud.dev/blob"
	"golang.org/x/sync/errgroup"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	state = path.Join(xdg.StateHome, "sindri")
	cache = path.Join(xdg.CacheHome, "sindri")
)

func NewSindri() *cobra.Command {
	var (
		port         int
		bucket       string
		certFile     string
		keyFile      string
		imageBuilder = &dagger.ImageBuilder{
			WorkDir: state,
		}
		registry   = &sindri.PullRegistry{ImageBuilder: imageBuilder}
		reconciler = &controller.DedicatedServerReconciler{}
		cmd        = &cobra.Command{
			Use: "sindri",
			RunE: func(cmd *cobra.Command, _ []string) error {
				var (
					eg, ctx = errgroup.WithContext(cmd.Context())
					log     = logutil.SloggerFrom(ctx)
					srv     = &http.Server{
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

				if reconciler.Registry == "" {
					log.Warn("not running controller due to missing --registry")

					return eg.Wait()
				}

				cfg, err := ctrl.GetConfig()
				if err != nil {
					return err
				}

				scheme, err := api.NewScheme()
				if err != nil {
					return err
				}

				mgr, err := ctrl.NewManager(cfg, ctrl.Options{
					Scheme:                        scheme,
					LeaderElectionID:              "ah7dchz8.sindri.frantj.cc",
					LeaderElectionReleaseOnCancel: true,
				})
				if err != nil {
					return err
				}

				if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
					return err
				}

				if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
					return err
				}

				if err := reconciler.SetupWithManager(mgr); err != nil {
					return err
				}

				eg.Go(func() error {
					log.Info("reconciling...")

					return mgr.Start(ctx)
				})

				return eg.Wait()
			},
		}
	)

	cmd.Flags().IntVar(&port, "port", 5000, "Port to listen on")
	cmd.Flags().StringVar(&bucket, "bucket", fmt.Sprintf("file://%s?create_dir=1&no_tmp_dir=1", cache), "Bucket URL")

	cmd.Flags().StringVar(&certFile, "tls-crt", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "tls-key", "", "TLS private key file")
	cmd.MarkFlagsRequiredTogether("tls-crt", "tls-key")

	cmd.Flags().StringVar(&imageBuilder.ModulesDirectory, "modules-directory", os.Getenv("SINDRI_MODULES_DIRECTORY"), "Path to Sindri's Dagger modules")
	cmd.Flags().StringVar(&imageBuilder.ModulesRef, "modules-git-ref", os.Getenv("SINDRI_MODULES_GIT_REF"), "Git ref of Sindri's Dagger modules")

	cmd.Flags().StringVar(&reconciler.Registry, "registry", "", "An address at which the cluster that Sindri is reconciling against can reach Sindri's container registry")

	cmd.Flags().BoolVar(&registry.UseSignedURLs, "use-signed-urls", false, "Use signed URLs to distribute layers")

	return cmd
}
