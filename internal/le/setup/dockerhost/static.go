// Design: docs/architecture/appliance/docker-host.md -- the static binaries from pinned source
// Related: build.go -- the build that asks for them

package dockerhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// staticBinaryNames are the files the builder container leaves in /out, each
// a fully static musl binary the image carries under /usr/lib/ze-docker-host.
var staticBinaryNames = []string{"sshd", "sshd-session", "ssh-keygen", "sftp-server", sudoBinary, "xtables-legacy-multi"}

// dockerCLI is the Docker client, and the name of Docker's pin and runtime.
const dockerCLI = "docker"

// sudoBinary is the static binary the image carries setuid root.
const sudoBinary = "sudo"

// staticSourceNames are the source pins the recipe builds, each named by a
// shell variable `<name>_tar` holding its tarball's file name.
var staticSourceNames = []string{"openssh", "sudo", "iptables"}

// safeFileName is what a tarball name must look like to be quoted into the
// recipe: it comes from the pin file's URL.
var safeFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// staticRecipe builds OpenSSH (no OpenSSL, A-7), sudo (no PAM) and legacy
// iptables (A-5, D-5) as fully static musl binaries in the pinned Alpine
// builder, a build tool only (D-13, A-12). libtool drops `-static` when it
// links sudo and iptables; only its own `-all-static`, given to make,
// survives. The sources arrive sum-checked in /in, never fetched here.
const staticRecipe = `set -eux
apk add --no-cache build-base linux-headers file pkgconf perl
mkdir -p /build
cd /build && tar -xzf "/in/$openssh_tar" && cd "${openssh_tar%.tar.gz}"
./configure --prefix=/usr --sysconfdir=/etc/ssh --libexecdir=/usr/lib/ze-docker-host \
  --with-privsep-path=/var/empty --with-privsep-user=sshd --without-openssl --without-zlib \
  --with-default-path=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  --with-superuser-path=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  --with-pid-dir=/run --disable-utmp --disable-wtmp --disable-lastlog --disable-strip \
  LDFLAGS=-static
make -j"$(nproc)" sshd sshd-session ssh-keygen sftp-server
strip sshd sshd-session ssh-keygen sftp-server
cp sshd sshd-session ssh-keygen sftp-server /out/
cd /build && tar -xzf "/in/$sudo_tar" && cd "${sudo_tar%.tar.gz}"
./configure --disable-pie --prefix=/usr --sysconfdir=/etc --without-pam \
  --disable-shared --enable-static-sudoers --disable-nls --without-selinux --disable-log-server \
  --disable-log-client --with-rundir=/run/sudo --with-vardir=/var/lib/sudo --with-noexec=no \
  --disable-zlib --with-secure-path
make -j"$(nproc)" LDFLAGS=-all-static
strip src/sudo
cp src/sudo /out/sudo
cd /build && tar -xJf "/in/$iptables_tar" && cd "${iptables_tar%.tar.xz}"
./configure --prefix=/usr --disable-nftables --disable-shared --enable-static --disable-devel \
  --disable-libipq --disable-bpf-compiler --disable-nfsynproxy --with-xtlibdir=/usr/lib/xtables
make -j"$(nproc)" LDFLAGS=-all-static
strip iptables/xtables-legacy-multi
cp iptables/xtables-legacy-multi /out/
for binary in /out/*; do file "$binary"; file "$binary" | grep -Eq 'statically linked|static-pie linked'; done
`

// containerRun is one run of the builder container.
type containerRun struct {
	// name is the container's name, removed by name after the run.
	name string
	// image is the builder image, pinned by digest.
	image string
	// arch is the image architecture the binaries are built for.
	arch string
	// inDir holds the source tarballs, mounted read-only at /in.
	inDir string
	// outDir receives the binaries, mounted at /out.
	outDir string
	// script is the shell program the container runs.
	script string
}

// staticBinaries answers the cache directory holding staticBinaryNames for
// the arch, running the builder container when the entry is absent. The
// entry's key digests the builder, the arch, the recipe and every source
// pin, so a changed pin or recipe builds afresh.
func (b *builder) staticBinaries(ctx context.Context, req buildRequest, sources map[string]string) (string, error) {
	assignments, err := recipeAssignments(req.pins, sources)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = io.WriteString(hash, req.pins.Builder()+"\x00"+req.arch+"\x00"+staticRecipe+"\x00") //nolint:errcheck // a hash write never fails
	for _, pin := range req.pins.Sources() {
		_, _ = io.WriteString(hash, pin.Name+"\x00"+pin.Version+"\x00"+pin.SHA256+"\x00") //nolint:errcheck // a hash write never fails
	}
	dir := filepath.Join(b.cacheDir, req.arch, "static-"+hex.EncodeToString(hash.Sum(nil))[:16])
	if hasEveryBinary(dir) {
		return dir, nil
	}

	partial := dir + ".partial-" + strconv.Itoa(os.Getpid())
	if err := os.MkdirAll(partial, 0o755); err != nil { //nolint:gosec // a cache directory in the user's home
		return "", fmt.Errorf("create %s: %w", partial, err)
	}
	defer os.RemoveAll(partial) //nolint:errcheck // gone already once renamed into place

	run := containerRun{
		name:   "ze-docker-host-build-" + strconv.Itoa(os.Getpid()),
		image:  req.pins.Builder(),
		arch:   req.arch,
		inDir:  filepath.Dir(sources[staticSourceNames[0]]),
		outDir: partial,
		script: assignments + staticRecipe,
	}
	if err := b.runContainer(ctx, run); err != nil {
		return "", err
	}
	if !hasEveryBinary(partial) {
		return "", fmt.Errorf("the builder container exited 0 but %s lacks one of %s. Fix: read the container's output above, "+
			"correct staticRecipe in internal/le/setup/dockerhost/static.go", partial, strings.Join(staticBinaryNames, ", "))
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("clear %s: %w", dir, err)
	}
	if err := os.Rename(partial, dir); err != nil {
		return "", fmt.Errorf("move the static binaries into %s: %w", dir, err)
	}
	return dir, nil
}

// recipeAssignments are the shell lines naming each source tarball for the
// recipe. Every source the recipe builds must be pinned, all in one
// directory, with a name safe to quote.
func recipeAssignments(pins *Pins, sources map[string]string) (string, error) {
	var lines strings.Builder
	directory := ""
	for _, name := range staticSourceNames {
		path, found := sources[name]
		if !found {
			return "", fmt.Errorf("pin file %s has no source named %s, which the image's static build needs. "+
				"Fix: add a %s entry under \"sources\" with its url and sha256", pins.Path(), name, name)
		}
		base := filepath.Base(path)
		if !safeFileName.MatchString(base) {
			return "", fmt.Errorf("pin file %s: source %s file name %q holds characters the build will not quote. "+
				"Fix: name a URL whose file name is letters, digits, dots, dashes, plus signs or underscores", pins.Path(), name, base)
		}
		if directory == "" {
			directory = filepath.Dir(path)
		}
		if filepath.Dir(path) != directory {
			return "", fmt.Errorf("BUG: source %s fetched to %s, outside %s", name, path, directory)
		}
		lines.WriteString(name + "_tar='" + base + "'\n")
	}
	return lines.String(), nil
}

// hasEveryBinary answers whether dir holds every static binary.
func hasEveryBinary(dir string) bool {
	for _, name := range staticBinaryNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// containerRemoveTimeout bounds the removal of the builder container.
const containerRemoveTimeout = time.Minute

// runBuilderContainer runs the recipe in the builder container on the Docker
// daemon the docker CLI reaches by its own environment (DOCKER_HOST or the
// current context): the builder is a build tool, not a lab, so the lab
// endpoint rules do not choose its daemon. The container is removed by name
// afterwards, whatever the outcome, so nothing of the run stays on the daemon.
func runBuilderContainer(ctx context.Context, run containerRun) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("the docker CLI is not on PATH, and the static binaries are built in a container. " +
			"Fix: install the docker CLI (brew install docker) and point it at a running daemon")
	}
	defer removeContainer(run.name)

	argv := []string{dockerCLI, "run", "--name", run.name, "--platform", "linux/" + run.arch,
		"--volume", run.inDir + ":/in:ro", "--volume", run.outDir + ":/out",
		run.image, "sh", "-c", run.script}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv built here from the pins
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("the builder container %s failed: %w (its output is above). "+
			"Fix: correct what the build names, then run `./le setup docker-host build` again", run.name, err)
	}
	return nil
}

// removeContainer removes the builder container by name. A container that is
// already gone is the expected case after a failed create.
func removeContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), containerRemoveTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "rm", "--force", name).CombinedOutput() //nolint:gosec // the name this run gave
	if err != nil && !strings.Contains(string(output), "No such container") {
		_, _ = os.Stderr.WriteString("docker-host: could not remove the builder container " + name + ": " + //nolint:errcheck // a warning line
			strings.TrimSpace(string(output)) + ". Fix: docker rm --force " + name + "\n")
	}
}
