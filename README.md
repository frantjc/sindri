# sindri

Sindri is a toolkit for turning Steamapps into containers. This repository also houses tools built from this toolkit.

## valheimw

`valheimw` is a wrapper around the Valheim server.

On start up, it installs the latest version of the specified branch of Valheim (public by default), any given [thunderstore.io](https://valheim.thunderstore.io/) mods and BepInEx to load them.

It runs an HTTP server alongside the Valheim server which provides endpoints to download the mods in use, the world's `.db` and `.fwl` files (and information from them), or go to its [valheim-map.world](https://valheim-map.world/) page.

Lastly, it documents all arguments that can be passed to Valheim's server.

```sh
valheimw --help
```

See [examples/valheimw](examples/valheimw).

## boiler

`boiler` is a read-only container registry for pulling images with Steam apps installed on them. The base of the images is `debian:stable-slim`. Images are non-root and `steamcmd` is never installed on them, so there's no leftover files from it on the image's filesystem or in its layers. Images are built on-demand rather than being stored, waiting to be pulled.

The image's tag maps to the Steam app's branch, except the specific case of the default tag "latest" which maps to the default Steam app branch "public".

```sh
docker compose up boiler
```

```sh
docker run --rm localhost:8080/896660
```

See [examples/boiler](examples/boiler).

## boil

`boil` is the CLI version of `boiler`. It builds an image from a given base and installs the specified Steam app onto it. Since `steamcmd` is never installed on the images, there's no leftover files from it on the image's filesystem or in its layers.

```sh
boil --base debian:stable-slim 896660 --platformtype linux | docker load
docker run --rm boil.frantj.cc/896660:public
```

## mist

`mist` is a CLI intended for use in Dockerfiles to install Steam apps, Steam Workshop items, and [thunderstore.io](https://thunderstore.io/) mods.

```dockerfile
FROM debian:stable-slim
COPY --from=ghcr.io/frantjc/mist /mist /usr/local/bin
RUN mist steamapp://896660 /root/valheim
RUN mist thunderstore://denikson/BepInExPack_Valheim /root/valheim
RUN mist thunderstore://RandyKnapp/EquipmentAndQuickSlots /root/valheim/BepInEx/plugins
RUN mist --clean
RUN rm /usr/local/bin/mist
```

## corekeeper

`corekeeper` is a container image built by `boiler` and layered upon to satisfy the Core Keeper server's additional dependecies. See [examples/corekeeper](examples/corekeeper).
