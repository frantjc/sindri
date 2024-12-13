package command

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/frantjc/go-steamcmd"
	"github.com/frantjc/sindri/contreg"
	"github.com/frantjc/sindri/distrib"
	"github.com/frantjc/sindri/internal/layerutil"
	"github.com/frantjc/sindri/steamapp"
	xslice "github.com/frantjc/x/slice"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

//go:embed image.tar
var imageTar []byte

func NewBoil() *cobra.Command {
	var (
		addr     string
		registry = &distrib.SteamappPuller{
			Dir:   "/boil/steamapp",
			User:  "boil",
			Group: "boil",
		}
		cmd = &cobra.Command{
			Use:           "boil",
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				srv := &http.Server{
					Addr:              addr,
					ReadHeaderTimeout: time.Second * 5,
					BaseContext: func(_ net.Listener) context.Context {
						return logr.NewContextWithSlogLogger(cmd.Context(), slog.Default())
					},
					Handler: distrib.Handler(registry),
				}

				l, err := net.Listen("tcp", addr)
				if err != nil {
					return err
				}
				defer l.Close()

				registry.Base, err = tarball.Image(func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(imageTar)), nil
				}, nil)
				if err != nil {
					return err
				}

				return srv.Serve(l)
			},
		}
	)

	cmd.SetVersionTemplate("{{ .Name }}{{ .Version }} " + runtime.Version() + "\n")

	cmd.Flags().StringVar(&addr, "addr", ":8080", "address")

	cmd.Flags().StringVar(&registry.Username, "username", "", "Steam username")
	cmd.Flags().StringVar(&registry.Password, "password", "", "Steam password")

	return cmd
}

func NewBuild() *cobra.Command {
	var (
		output, rawRef, rawBaseImageRef string
		beta, betaPassword              string
		username, password              string
		dir                             string
		cmd                             = &cobra.Command{
			Use:           "build",
			Args:          cobra.ExactArgs(1),
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(cmd *cobra.Command, args []string) error {
				var (
					imageW  = cmd.OutOrStdout()
					updateW = cmd.ErrOrStderr()
				)

				if !xslice.Includes([]string{"", "-"}, output) {
					var err error
					imageW, err = os.Create(output)
					if err != nil {
						return err
					}

					updateW = cmd.OutOrStdout()
				}

				appID, err := strconv.Atoi(args[0])
				if err != nil {
					return err
				}

				var (
					ctx     = cmd.Context()
					updateC = make(chan v1.Update)
				)
				go func() {
					for update := range updateC {
						_, _ = fmt.Fprintf(updateW, "%d/%d\n", update.Complete, update.Total)
					}
				}()
				defer close(updateC)

				var (
					opts = []steamapp.Opt{
						steamapp.WithAccount(username, password),
						steamapp.WithBeta(beta, betaPassword),
					}
					image = empty.Image
				)

				if rawBaseImageRef != "" {
					baseImageRef, err := name.ParseReference(rawBaseImageRef)
					if err != nil {
						return err
					}

					image, err = contreg.DefaultClient.Pull(ctx, baseImageRef)
					if err != nil {
						return err
					}
				}

				if rawRef == "" {
					prompt, err := steamcmd.Start(ctx)
					if err != nil {
						return err
					}

					appInfo, err := prompt.AppInfoPrint(ctx, appID)
					if err != nil {
						return err
					}

					if err = prompt.Close(ctx); err != nil {
						return err
					}

					branchName := steamapp.DefaultBranchName
					if beta != "" {
						branchName = beta
					}

					rawRef = fmt.Sprintf(
						"boil.frantj.cc/%d:%s",
						appInfo.Common.GameID,
						branchName,
					)
				}

				cfgf, err := image.ConfigFile()
				if err != nil {
					return err
				}

				cfg, err := steamapp.ImageConfig(ctx, appID, &cfgf.Config, append(opts, steamapp.WithInstallDir(dir))...)
				if err != nil {
					return err
				}

				image, err = mutate.Config(image, *cfg)
				if err != nil {
					return err
				}

				layer, err := layerutil.ReproducibleBuildLayerInDirFromOpener(
					func() (io.ReadCloser, error) {
						return steamapp.Open(
							ctx,
							appID,
							opts...,
						)
					},
					dir,
					"", "",
				)
				if err != nil {
					return err
				}

				image, err = mutate.AppendLayers(image, layer)
				if err != nil {
					return err
				}

				ref, err := name.ParseReference(rawRef)
				if err != nil {
					return err
				}

				if err := tarball.Write(ref, image, imageW, tarball.WithProgress(updateC)); err != nil {
					return err
				}

				return nil
			},
		}
	)

	cmd.SetVersionTemplate("{{ .Name }}{{ .Version }} " + runtime.Version() + "\n")

	cmd.Flags().StringVarP(&output, "output", "o", "", "file to write the image to (default stdout)")
	cmd.Flags().StringVarP(&rawRef, "ref", "r", "", "ref to write the image as (default boil.frantj.cc/<steamappid>:<branch>)")
	cmd.Flags().StringVarP(&rawBaseImageRef, "base", "b", "", "base image to build upon (default scratch)")

	cmd.Flags().StringVar(&beta, "beta", "", "Steam beta branch")
	cmd.Flags().StringVar(&betaPassword, "beta-password", "", "Steam beta password")

	cmd.Flags().StringVar(&dir, "dir", "/boil/steamapp", "Steam app install directory")

	cmd.Flags().StringVar(&username, "username", "", "Steam username")
	cmd.Flags().StringVar(&password, "password", "", "Steam password")

	return cmd
}
