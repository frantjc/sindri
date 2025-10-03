// A generated module for AI functions

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
	"github.com/frantjc/sindri/dagger/modules/llm/internal/dagger"
)

type LLM struct{}

const (
	group = "sindri"
	user  = group
	owner = user + ":" + group
)

func (m *LLM) Container(
	ctx context.Context,
	appID int,
	// +optional
	// +default="public"
	branch,
	// +optional
	betaPassword string,
	// +optional
	launchType string,
) (*dagger.Container, error) {
	rawAppInfo, err := dag.Steamcmd().AppInfoPrint(ctx, appID)
	if err != nil {
		return nil, err
	}

	appInfo := &steamcmd.AppInfo{}

	if err := vdf.NewDecoder(strings.NewReader(rawAppInfo)).Decode(appInfo); err != nil {
		return nil, err
	}

	steamappDirectoryPath := path.Join("/opt/sindri/steamapps", fmt.Sprint(appID))

	steamappDirectory := dag.Steamcmd().AppUpdate(appID, dagger.SteamcmdAppUpdateOpts{
		Branch:       branch,
		BetaPassword: betaPassword,
	})

	// TODO(frantjc): Prefer Wolfi, but LLMs are not as good with it, understandably.
	// possiblePackageLists := dag.Wolfi().
	// 	Container().
	// 	WithFile(
	// 		"/tmp/APKINDEX.tar.gz",
	// 		dag.HTTP("https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz"),
	// 	).WithExec([]string{
	// 		"mkdir",
	// 		"-p",
	// 		"/pkgs",
	// 	}).
	// 	WithExec([]string{"sh", "-c", `
	// 		tar -xzOf /tmp/APKINDEX.tar.gz APKINDEX \
	// 			| grep '^P:' \
	// 			| grep -E '(lib|ca-certificates-bundle)' \
	// 			| cut -d: -f2 \
	// 			| sort -u \
	// 			| split -b 9999 - /pkgs/packages_
	// 	`}).
	// 	Directory("/pkgs")

	launch, found := getLaunch(appInfo, func(launch *steamcmd.AppInfoConfigLaunch) bool {
		return isLinux(launch) && (launchType == launch.Type || (launchType == "" && slices.Contains([]string{"server", "default"}, launch.Type)))
	})
	if !found {
		return nil, fmt.Errorf("did not find launch config")
	}

	rawPackages, err := dag.LLM().
		WithEnv(
			dag.Env().
				WithFileInput("app_info", dag.File("app_info.vdf", rawAppInfo), "Steam app metadata").
				WithDirectoryInput("steamapp", steamappDirectory, "Game installation files").
				// WithDirectoryInput("packages", possiblePackageLists, "Available Wolfi packages").
				WithStringOutput("result", "JSON array of minimal required package names"),
		).
		WithPrompt("Analyze the Steam app in $app_info and game files in $steamapp and output a JSON array of minimal APT package names needed to run this application").
		Env().
		Output("result").
		AsString(ctx)
	if err != nil {
		return nil, err
	}

	packages := []string{}

	if err := json.NewDecoder(strings.NewReader(rawPackages)).Decode(&packages); err != nil {
		return nil, err
	}

	args := []string{}
	if len(launch.Arguments) > 0 {
		args = strings.Split(launch.Arguments, " ")
	}

	return dag.Debian().
		Container(packages).
		// WithExec([]string{"addgroup", "-S", group}).
		// WithExec([]string{"adduser", "-S", user, group}).
		WithExec([]string{"groupadd", "-r", group}).
		WithExec([]string{"useradd", "-m", "-g", group, "-r", user}).
		WithDirectory(
			steamappDirectoryPath,
			steamappDirectory,
			dagger.ContainerWithDirectoryOpts{Owner: owner},
		).
		WithUser(user).
		WithWorkdir(steamappDirectoryPath).
		WithEntrypoint([]string{path.Join(steamappDirectoryPath, launch.Executable)}).
		WithDefaultArgs(args), nil
}

var (
	isLinux = supportsOS("linux")
)

func supportsOS(os string) func(launch *steamcmd.AppInfoConfigLaunch) bool {
	return func(launch *steamcmd.AppInfoConfigLaunch) bool {
		return strings.Contains(launch.Config.OSList, os)
	}
}

func getLaunch(appInfo *steamcmd.AppInfo, f func(launch *steamcmd.AppInfoConfigLaunch) bool) (*steamcmd.AppInfoConfigLaunch, bool) {
	for _, launch := range appInfo.Config.Launch {
		if f(&launch) {
			return &launch, true
		}
	}

	return nil, false
}
