// A generated module for Steamcmd functions

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/steamapps/internal/dagger"
	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
)

func steamcmdContainer() *dagger.Container {
	return dag.Container().
		From("steamcmd/steamcmd")
}

func appInfoPrint(
	ctx context.Context,
	appID int,
) (*steamcmd.AppInfo, error) {
	appInfoPrintArgs, err := steamcmd.Args(nil,
		steamcmd.Login{},
		steamcmd.AppInfoPrint(appID),
		steamcmd.Quit,
	)
	if err != nil {
		return nil, err
	}

	steamcmdAppInfoPrintExec := append([]string{"steamcmd"}, appInfoPrintArgs...)
	cache := fmt.Sprint(time.Now().Unix())

	rawAppInfo, err := steamcmdContainer().
		WithEnvVariable("_SINDRI_CACHE", cache).
		WithExec(steamcmdAppInfoPrintExec).
		CombinedOutput(ctx)
	if err != nil {
		return nil, err
	}

	appInfoStartTokenIndex := strings.Index(rawAppInfo, "{")
	if appInfoStartTokenIndex == -1 {
		return nil, fmt.Errorf("app_info_print did not output VDF")
	}

	appInfoEndTokenIndex := strings.LastIndex(rawAppInfo[appInfoStartTokenIndex:], "}")
	if appInfoEndTokenIndex == -1 {
		return nil, fmt.Errorf("app_info_print did not output VDF")
	}

	appInfo := &steamcmd.AppInfo{}

	if err := vdf.NewDecoder(strings.NewReader(rawAppInfo[appInfoStartTokenIndex : appInfoStartTokenIndex+appInfoEndTokenIndex])).Decode(appInfo); err != nil {
		return nil, err
	}

	return appInfo, nil
}

// TODO(frantjc): Split this up into multiple layers using depots (only when auth is passed: depots required auth).
func appUpdate(
	ctx context.Context,
	appID int,
	branch string,
	betaPassword string,
	platformType steamcmd.PlatformType,
) (*dagger.Directory, *steamcmd.AppInfo, error) {
	appInfo, err := appInfoPrint(ctx, appID)
	if err != nil {
		return nil, nil, err
	}

	appInfoRequestArgs, err := steamcmd.Args(nil,
		steamcmd.Login{},
		steamcmd.AppInfoRequest(appID),
		steamcmd.Quit,
	)
	if err != nil {
		return nil, nil, err
	}

	steamappDirectoryPath := "/out"

	appUpdateArgs, err := steamcmd.Args(nil,
		steamcmd.ForceInstallDir(steamappDirectoryPath),
		steamcmd.Login{},
		steamcmd.ForcePlatformType(platformType),
		steamcmd.AppUpdate{
			AppID:        appID,
			Beta:         branch,
			BetaPassword: betaPassword,
		},
		steamcmd.Quit,
	)
	if err != nil {
		return nil, nil, err
	}

	cache := branch
	if depot, ok := appInfo.Depots.Branches[branch]; ok {
		cache = fmt.Sprint(depot.TimeUpdated)
	}

	steamcmdAppInfoRequestExec := append([]string{"steamcmd"}, appInfoRequestArgs...)
	steamcmdAppUpdateExec := append([]string{"steamcmd"}, appUpdateArgs...)

	container, err := steamcmdContainer().
		WithEnvVariable("_SINDRI_CACHE", cache).
		// NB: Try to avoid exit 8 from "Missing configuration" error.
		WithExec(steamcmdAppInfoRequestExec).
		// TODO(franjc): Why does this cache miss sometimes even though the previous exec hit?
		WithExec(steamcmdAppUpdateExec).
		Sync(ctx)
	if err != nil {
		return nil, nil, err
	}

	return container.Directory(steamappDirectoryPath), appInfo, nil
}
