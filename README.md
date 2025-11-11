# sindri [![CI](https://github.com/frantjc/sindri/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/frantjc/sindri/actions) [![godoc](https://pkg.go.dev/badge/github.com/frantjc/sindri.svg)](https://pkg.go.dev/github.com/frantjc/sindri) [![goreportcard](https://goreportcard.com/badge/github.com/frantjc/sindri)](https://goreportcard.com/report/github.com/frantjc/sindri)

Sindri is read-only container registry that builds images on-demand, as they are pulled.

## use cases

- You're on a platform team providing base images for various languages for application teams, but you can't keep track of what versions application teams are using.
- You want to make builds available to users of a project, but they mostly go unused (e.g. nightly builds).
- You're on a platform team that wants to provide base images that allow your users to mix and match dependencies.

Generally, any time you want a container image to be available if it is needed, but you don't want to use the compute to build it and the disk to store it in case it isn't, Sindri can help. Read on for the use cases that inspired me to building Sindri.

## how it works

Sindri uses [Dagger](https://dagger.io/) to build images on-demand. It can store the built images in various backends, such as in OCI registries, on disk, and in s3-compatible buckets.

### dagger

Any Dagger module "sindri" that exposes a function "container" which takes two strings as arguments ["name" for the `<name>` and "reference" for the `<reference>`](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests) and returns a Dagger container is supported--just run `sindri` from the module's directory. See [interface](modules/interface/) for a minimal example, and the rest of the [modules](modules/) for some cool use-cases. Following is a list of example uses of Sindri's in-tree modules, plus instructions on how to use your own.

#### steamapps

Run Sindri with the [steamapps](modules/steamapps) module for building containers for Steamapp dedicated servers:

```sh
docker run --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri --debug
```

Then start pulling container images from Sindri:

```sh
docker pull --tls-verify=false localhost:5000/corekeeper
```

#### wolfi

Run Sindri with the [wolfi](modules/wolfi) module for building Wolfi containers with pre-installed packages:

```sh
docker run --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri:wolfi --debug
```

Then start pulling container images from Sindri:

```sh
docker pull --tls-verify=false localhost:5000/go-1.25
```

#### git

Run Sindri with the [git](modules/git) module for building containers from Git repositories' Dockerfiles:

```sh
docker run --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri:git --debug
```

Then start pulling container images from Sindri:

```sh
docker pull --tls-verify=false localhost:5000/github.com/frantjc/sindri/testdata
```

#### bring your own

Run Sindri from the directory of your module that implements Sindri's Dagger module [interface](modules/interface/main.go):

```sh
docker run --volume `pwd`:/home/sindri/.config/sindri/module --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri --debug
```

Then start pulling container images built by your module from Sindri:

```sh
docker pull --tls-verify=false localhost:5000/<name>:<reference>
```

### storage

Sindri supports multiple storage backends for cacheing and serving container image manifests and blobs after they are exported from Dagger. All backends can be used via a [gocloud.dev URL](https://gocloud.dev/concepts/urls/).

#### [gocloud.dev/blob.Bucket](https://gocloud.dev/howto/blob/)

> An additional query parameter is supported by Sindri for opening buckets: `use_signed_urls=true`. Use this to avoid proxying container image content through Sindri for buckets that support it. This feature should reduce cost and improve performance.

Run Sindri using an s3 bucket as its storage backend:

```sh
docker run --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri --debug --backend s3://<bucket>?use_signed_urls=true
```

Run Sindri using a local directory as its storage backend:

```sh
docker run --volume /tmp:/tmp --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri --debug --backend file:///tmp
```

The same pattern follows for any other `gocloud.dev/blob` drivers.

#### [ghcr.io](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

Run Sindri using ghcr.io as its storage backend:

```sh
docker run --env GITHUB_TOKEN=ghp_xxx --publish 5000:5000 --detach --rm ghcr.io/frantjc/sindri --debug --backend registry://ghcr.io/<org>/<repo>
```

> ghcr.io creates new container packages as private which must be manually changed to public as of writing. This will cause the first pull of any `<name>` from Sindri using ghcr.io as its storage backend to fail.

## thx

- [Nixery](https://nixery.dev/) for the idea.
- [Chainguard's registry-redirect](https://github.com/chainguard-dev/registry-redirect) for providing a very useful reference for implementing the [registry backend](backend/registry/).
- [Dagger](https://github.com/dagger/dagger) for being awesome.
