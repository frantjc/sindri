import { Steamapp } from "../client";

export function generateContainerDefinition(
  steamapp: Steamapp | undefined,
): string {
  if (!steamapp) return "";

  const isBeta = steamapp.branch && steamapp.branch !== "public";
  if (
    isBeta &&
    (!steamapp.beta_password || steamapp.beta_password.length === 0)
  ) {
    throw new Error(
      "Beta branch requires a beta_password, but none was provided",
    );
  }

  const betaBranch = isBeta ? ` -beta ${steamapp.branch}` : "";
  const betaPassword = isBeta ? ` -betapassword ${steamapp.beta_password}` : "";

  const lines = [
    "FROM steamcmd/steamcmd AS steamcmd",
    "RUN groupadd --system steam \\",
    "  && useradd --system --gid steam --shell /bin/bash --create-home steam \\",
    "  && steamcmd \\",
    "    +force_install_dir /mnt \\",
    "    +login anonymous \\",
    `    @sSteamCmdForcePlatformType ${steamapp.platform_type} \\`,
    `    +app_update ${steamapp.app_id}${betaBranch}${betaPassword} \\`,
    "    +quit",
    "",
    "FROM " + steamapp.base_image,
    steamapp.apt_packages && steamapp.apt_packages.length
      ? [
          "RUN apt-get update -y \\",
          "  && apt-get install -y --no-install-recommends \\",
          ...steamapp.apt_packages.map((pkg) => `    ${pkg} \\`),
          "  && rm -rf /var/lib/apt/lists/* \\",
          "  && apt-get clean",
        ].join("\n")
      : "",
    "RUN groupadd --system steam \\",
    "  && useradd --system --gid steam --shell /bin/bash --create-home steam",
    "COPY --from=steamcmd --chown=steam:steam /mnt /home/steam",
    steamapp.execs && steamapp.execs.length
      ? [
          "RUN " + steamapp.execs[0] + (steamapp.execs.length > 1 ? " \\" : ""),
          ...steamapp.execs
            .slice(1)
            .map((exec, i) =>
              i === steamapp.execs!.length - 2
                ? `  && ${exec}`
                : `  && ${exec} \\`,
            ),
        ].join("\n")
      : "",
    "USER steam",
    steamapp.entrypoint && steamapp.entrypoint.length
      ? `ENTRYPOINT [${steamapp.entrypoint.map((e) => `"${e}"`).join(", ")}]`
      : "",
    steamapp.cmd && steamapp.cmd.length
      ? `CMD [${steamapp.cmd.map((c) => `"${c}"`).join(", ")}]`
      : "",
  ];

  return lines.join("\n");
}
