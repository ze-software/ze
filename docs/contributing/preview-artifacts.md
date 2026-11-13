# Preparing a Ze preview

A preview lets a reviewer evaluate a particular source tree before a signed
release exists. It carries a preview version, commit, dirty flag, selected input
hashes and checksums. The commit, dirty flag and build date describe the checkout
at preparation's start. A concurrent commit or generated output does not change
that captured identity when the selected input bytes, population, tags and
toolchain remain identical through native compilation and archive creation.
Preparing it does
not publish a release or make a download
available on the website.
<!-- source: internal/le/build/preview/preview.go -- Manifest and prepare -->

From the main checkout, choose a new output directory:

```console
./le --name preview build preview run output /absolute/path/to/new-preview images
./le --name preview build preview check path /absolute/path/to/new-preview
```

The command builds static Linux amd64 and arm64 daemon binaries with the default
features and Ze's patched Go toolchain. Each archive contains `ze`, its license
and installation instructions. `manifest.json` records the targets, source
identity and artifact hashes; `SHA256SUMS` also covers the manifest. Existing
output directories are refused. The complete source identity is checked after
both native binaries and archives are complete, before image packaging starts.
An input change during that phase refuses the bundle and names the first
differing input when available. Later Go source edits do not change the
completed outputs or their captured identity.
<!-- source: internal/le/build/preview/preview.go -- prepare; internal/le/build/compile/daemon.go -- BuildDaemon -->

Both commands print the bundle directory, manifest and checksum paths, preview
identity, source input count and artifact inventory. The complete per-file
source hashes and image version output remain in `manifest.json`.
<!-- source: internal/le/build/preview/result.go -- Result, summarizePreview, runPreview, checkPreview -->

The optional `images` keyword packages the already inspected native binary for
the Docker daemon's architecture in deployment and lab images. Each build
receives only that binary and `docker/Dockerfile.runtime`; it compiles no Go and
sends no source checkout or other lab's staged artifacts. Both preview images
carry the binary's exact selected tags, including `ze_distro` and its native
installation commands. The source-building `docker/Dockerfile` and
`docker/Dockerfile.lab` retain their own tag selection and do not add
`ze_distro`. Their commands and the preview images therefore have different
inventories. The preview manifest records the exact tags, immutable IDs, local
tags and architecture; each final image is exported as a Docker archive.
This proves neither a second image architecture nor a running BGP session.
Each staged runtime recipe is hashed before its Docker build and must match
the captured recipe input. A later restoration of the checkout cannot hide a
change already copied into a staged recipe.
<!-- source: internal/le/build/preview/images.go -- prepareImages -->
<!-- source: internal/le/interoplab/runtime_image.go -- StageRuntimeImage -->
<!-- source: docker/Dockerfile.runtime -- deployment and lab -->

The checker binds every Docker archive to one image record. It checks the
exported manifest/index or classic config identity, actual runtime OS and
architecture, blob hashes and sizes, and the ordered uncompressed filesystem
layer hashes. Docker compatibility metadata must agree with the OCI graph;
BuildKit attestations remain metadata for that one runtime platform. Filesystem
layers use raw tar or gzip; unsupported compression is refused explicitly.
These checks bind the export to the image whose version the producer ran; they
do not authenticate an unsigned bundle's sender.
<!-- source: internal/le/build/preview/docker_archive.go -- checkDockerArchive, imageGraph.walk, archiveContent.layer -->

Each regular file in the outer Docker archive must be dense and at most 16 GiB.
The checker refuses sparse outer files and larger declared sizes before hashing
their content. Filesystem layers inside those dense files can contain sparse
entries. JSON metadata is limited to 16 MiB, and uncompressed filesystem layers
are limited to 16 GiB.
<!-- source: internal/le/build/preview/docker_archive.go -- scanImageArchive, archiveContent.json, archiveContent.layer -->

Verify the checksums before extracting or installing a received bundle:

```console
sha256sum --check SHA256SUMS
tar -xzf ze-preview-<identity>-linux-amd64.tar.gz
./ze version
sudo ./ze install
sudo ze uninstall
```

Use the archive matching the machine's architecture. Installation creates the
native service; review the initial configuration and provision authentication
before exposing management access. See the [installation guide](../guide/ubuntu-build-install.md)
for configuration and service operation. Checksums detect changed files; an
unsigned preview requires a trusted transfer of the bundle and its identity.
<!-- source: internal/le/build/preview/archive.go -- archiveReadme; internal/le/build/preview/check.go -- check -->

Load an exported image, then give its recorded immutable ID a local tag:

```console
docker load --input ze-preview-<identity>-deployment-linux-amd64.docker.tar
docker tag '<image-id-from-manifest>' ze-local:preview
```

Copy the complete image ID, including its `sha256:` prefix, from the manifest.
Use the deployment image with the [Docker guide](../guide/docker.md), or load and tag
the lab image for [netlab and containerlab](../guide/netlab.md). Mount a configuration
and a persistent state directory using those guides; image archives contain no
per-device credentials or private appliance state.
<!-- source: internal/le/build/preview/images.go -- prepareImages -->

The manual `preview` GitHub workflow runs the same producer and checker and
uploads the bundle as an Actions artifact. Download requires access to that
workflow run; artifacts expire according to its retention setting. The workflow
has a read-only repository token and creates no release or registry publication.
<!-- source: .github/workflows/preview.yml -- workflow_dispatch and permissions -->

Appliance ISO and disk images need separate provisioning. A private image built
with a user's credentials is a private test artifact. Before publishing an
evaluation image, prepare a credential-free provisioning path or fresh
per-device credentials, retain the complete corresponding source and build
instructions, and satisfy the project's release signing and publication gates.
Boot and installation evidence for one architecture does not establish another
architecture's support. Do not include a seeded private ISO or disk image in this
preview bundle.
<!-- source: internal/appliance/cmd_iso.go -- installer ISO; internal/appliance/cmd_passwd.go -- appliance credentials; internal/le/build/preview/preview.go -- preview artifact population -->

The authenticated stable artifact and distribution contracts remain owned by
the release-build and release-distribution specifications. A prepared preview
must stay labeled preview until the owner authorizes the separate publication
step; website links must name artifacts that are actually available.
<!-- source: plan/release/delivery/spec-release-build.md -- Owner Direction; plan/release/delivery/spec-release-distribution.md -- owner amendments -->
