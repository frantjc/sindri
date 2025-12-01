// A Sindri module that builds Steamapp dedicated server containers.

// Enables building Docker containers for various Steamapp dedicated servers.
//
// The <name> parameter should match a supported game name (e.g. "palworld",
// "valheim", "satisfactory"), and the <reference> parameter specifies the
// server branch, with "latest" defaulting to "public".
//
// For example, `docker pull localhost:5000/valheim:publictest` will build
// a Valheim Dedicated Server container using the publictest branch.
//
// Supported games include Abiotic Factor, Astroneer, Core Keeper, Enshrouded,
// Palworld, Satisfactory, and Valheim. Unsupported names return an empty
// container which will cause an error when exporting or publishing.

package main

import (
	"context"
	"dagger/steamapps/internal/dagger"
	"fmt"
	"strings"

	"github.com/frantjc/go-steamcmd"
)

type Sindri struct{}

const (
	gid   = "1001"
	uid   = gid
	group = "sindri"
	user  = group
	owner = user + ":" + group
	home  = "/home/" + user

	defaultTag    = "public"
	defaultBranch = "latest"

	steamappDirectoryPath = home + "/.local/share/sindri/steamapp"
	steamClientSoPath     = steamappDirectoryPath + "/linux64/steamclient.so"
	steamClientSoLinkPath = "/usr/lib/x86_64-linux-gnu/steamclient.so"
)

var (
	steamworksSdkRedistLinuxInclude = []string{
		"linux64/**",
		"libsteamwebrtc.so",
		"steamclient.so",
	}
	defaultExclude = []string{
		"steamapps/",
		"steam_appid.txt",
	}
)

var (
	isWindows = supportsOS("windows")
	isLinux   = supportsOS("linux")
)

type appInfoLaunchConfigFilter func(launch *steamcmd.AppInfoConfigLaunch) bool

func supportsOS(os string) appInfoLaunchConfigFilter {
	return func(launch *steamcmd.AppInfoConfigLaunch) bool {
		return strings.Contains(launch.Config.OSList, os)
	}
}

func getLaunch(appInfo *steamcmd.AppInfo, f appInfoLaunchConfigFilter) (*steamcmd.AppInfoConfigLaunch, bool) {
	for _, launch := range appInfo.Config.Launch {
		if f(&launch) {
			return &launch, true
		}
	}

	return nil, false
}

func (m *Sindri) Container(ctx context.Context, name, reference string) (*dagger.Container, error) {
	if reference == "latest" {
		reference = "public"
	}

	switch name {
	case "abioticfactor":
		return new(Abioticfactor).Container(ctx, reference)
	case "astroneer":
		return new(Astroneer).Container(ctx, reference)
	case "corekeeper":
		return new(Corekeeper).Container(ctx, reference)
	case "enshrouded":
		return new(Enshrouded).Container(ctx, reference)
	case "palworld":
		return new(Palworld).Container(ctx, reference)
	case "satisfactory":
		return new(Satisfactory).Container(ctx, reference)
	case "valheim":
		return new(Valheim).Container(ctx, reference)
	}

	// TODO(frantjc): Maybe try to handle the generic case?
	return nil, fmt.Errorf("invalid name %s, try one of: abioticfactor, astroneer, corekeeper, enshrouded, palworld, satisfactory, valheim", name)
}
