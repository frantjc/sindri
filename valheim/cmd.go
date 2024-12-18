package valheim

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	xos "github.com/frantjc/x/os"
)

// NewCommand builds an *exec.Cmd for the Valheim executable
// in the given directory with the given options.
func NewCommand(ctx context.Context, dir string, opts *Opts) (*exec.Cmd, error) {
	if strings.Contains(opts.World, opts.Password) || len(opts.Password) < 5 {
		return nil, fmt.Errorf("-password must be >=5 characters and not contained within the world name")
	}

	if !filepath.IsAbs(dir) {
		var err error
		dir, err = filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
	}

	var (
		//nolint:gosec
		cmd = exec.CommandContext(
			ctx,
			filepath.Join(dir, "valheim_server.x86_64"),
			append(
				opts.ToArgs(),
				// Unclear if these do anything or where I got them,
				// but once upon a time I was lead to believe that
				// they improve performance.
				"-batchmode",
				"-nographics",
				"-screen-width", "640",
				"-screen-height", "480",
				"-screen-quality", "Fastest",
			)...,
		)
		ldLibraryPath = xos.JoinPath(
			os.Getenv("LD_LIBRARY_PATH"),
			filepath.Join(dir, "linux64"),
		)
	)

	cmd.Dir = dir

	if opts.BepInEx && false {
		var (
			doorstopLibs     = filepath.Join(cmd.Dir, "doorstop_libs")
			libdoorstop      = filepath.Join(doorstopLibs, "libdoorstop_x86") // ext added below
			bepInExPreloader = filepath.Join(cmd.Dir, "BepInEx/core/BepInEx.Preloader.dll")
			unstrippedCorlib = filepath.Join(cmd.Dir, "unstripped_corlib")
		)

		// TODO: Should this ever be _x64?
		// if strings.Contains(runtime.GOARCH, "amd64") {
		// 	libdoorstop += "_x64"
		// } else {
		// 	libdoorstop += "_x86"
		// }

		switch runtime.GOOS {
		case "windows":
			return nil, fmt.Errorf("%s incompatible with BepInEx", runtime.GOOS)
		case "darwin":
			libdoorstop += ".dylib"
		default:
			libdoorstop += ".so"
		}

		cmd.Env = os.Environ()

		if _, err := os.Stat(doorstopLibs); err == nil {
			cmd.Env = append(
				cmd.Env,
				"DOORSTOP_ENABLED=TRUE",
				fmt.Sprintf("DYLD_LIBRARY_PATH=%s", doorstopLibs),
			)
			ldLibraryPath = xos.JoinPath(ldLibraryPath, doorstopLibs)
		}

		if _, err := os.Stat(libdoorstop); err == nil {
			cmd.Env = append(
				cmd.Env,
				fmt.Sprintf("LD_PRELOAD=%s", xos.JoinPath(libdoorstop, os.Getenv("LD_PRELOAD"))),
				fmt.Sprintf("DYLD_INSERT_LIBRARIES=%s", libdoorstop),
			)
		}

		if _, err := os.Stat(bepInExPreloader); err == nil {
			cmd.Env = append(
				cmd.Env,
				fmt.Sprintf("DOORSTOP_INVOKE_DLL_PATH=%s", bepInExPreloader),
			)
		}

		if _, err := os.Stat(unstrippedCorlib); err == nil {
			cmd.Env = append(
				cmd.Env,
				fmt.Sprintf("DOORSTOP_CORLIB_OVERRIDE_PATH=%s", unstrippedCorlib),
			)
		}
	}

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("LD_LIBRARY_PATH=%s", ldLibraryPath),
	)

	return cmd, nil
}
