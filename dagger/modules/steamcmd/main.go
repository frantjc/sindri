// A generated module for Steamcmd functions

package main

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	vdf "github.com/frantjc/go-encoding-vdf"
	"github.com/frantjc/go-steamcmd"
	"github.com/frantjc/sindri/dagger/modules/steamcmd/internal/dagger"
)

type Steamcmd struct{}

func (m *Steamcmd) Container() *dagger.Container {
	return dag.Container().
		From("steamcmd/steamcmd")
}

func (m *Steamcmd) AppInfoPrint(
	ctx context.Context,
	appID int,
) (string, error) {
	appInfoPrintArgs, err := steamcmd.Args(nil,
		steamcmd.Login{},
		steamcmd.AppInfoPrint(appID),
		steamcmd.Quit,
	)
	if err != nil {
		return "", err
	}

	steamcmdAppInfoPrintExec := append([]string{"steamcmd"}, appInfoPrintArgs...)

	rawAppInfo, err := m.Container().
		WithExec(steamcmdAppInfoPrintExec).
		CombinedOutput(ctx)
	if err != nil {
		panic(err)
	}

	appInfoStartTokenIndex := strings.Index(rawAppInfo, "{")
	if appInfoStartTokenIndex == -1 {
		return "", fmt.Errorf("app_info_print did not output VDF")
	}

	appInfoEndTokenIndex := strings.LastIndex(rawAppInfo[appInfoStartTokenIndex:], "}")
	if appInfoEndTokenIndex == -1 {
		return "", fmt.Errorf("app_info_print did not output VDF")
	}

	return regexp.MustCompile(`\s+`).
		ReplaceAllString(
			rawAppInfo[appInfoStartTokenIndex:appInfoStartTokenIndex+appInfoEndTokenIndex],
			" ",
		), nil
}

type PlatformType = steamcmd.PlatformType

func (m *Steamcmd) AppUpdate(
	ctx context.Context,
	appID int,
	// +optional
	// +default="public"
	branch,
	// +optional
	betaPassword string,
	// +optional
	// +default="linux"
	platformType PlatformType,
) (*dagger.Directory, error) {
	rawAppInfo, err := m.AppInfoPrint(ctx, appID)
	if err != nil {
		return nil, err
	}

	appInfo := &steamcmd.AppInfo{}

	if err := vdf.NewDecoder(strings.NewReader(rawAppInfo)).Decode(appInfo); err != nil {
		return nil, err
	}

	steamappDirectoryPath := path.Join("/opt/sindri/steamapps", fmt.Sprint(appID))

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
		return nil, err
	}

	cache := branch
	if depot, ok := appInfo.Depots.Branches[branch]; ok {
		cache = fmt.Sprint(depot.TimeUpdated)
	}

	steamcmdAppUpdateExec := append([]string{"steamcmd"}, appUpdateArgs...)

	return m.Container().
		WithEnvVariable("_SINDRI_CACHE", cache).
		WithExec(steamcmdAppUpdateExec).
		Directory(steamappDirectoryPath), nil
}
