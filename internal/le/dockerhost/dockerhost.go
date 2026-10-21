// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package dockerhost names the Docker daemon socket a development command must
// talk to on macOS.
//
// On macOS every lab Docker daemon runs on Ze's kernel and Ze's system (owner
// decision M10, ai/META-DECISIONS.md): the `ze` Colima profile, which boots
// Ze's image (`./le setup docker-host start`). Colima serves that profile's
// daemon on ~/.colima/ze/docker.sock, so Environment names that socket as
// DOCKER_HOST whenever the operator has not named one. It never selects
// Colima's `default` profile, /var/run/docker.sock or the client's current
// context: each runs another kernel, and taking one when the `ze` socket is
// absent would run the labs on a daemon M10 rules out, so Environment refuses
// instead and names the command that starts the profile.
//
// A DOCKER_HOST the operator set is kept as it is. It is an explicit choice of
// daemon (the QEMU docker-lab's forwarded socket is one), and the Docker kernel
// check every lab runs first (`./le setup docker-kernel check`,
// interoplab.DockerKernel) stays the gate that refuses a daemon whose kernel
// lacks a feature.
package dockerhost

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// goosDarwin is the only platform whose labs run on a Colima profile.
const goosDarwin = "darwin"

// variable is the name a Docker client reads before it consults its context.
const variable = "DOCKER_HOST"

// ZeProfile is the Colima profile that boots Ze's image, and the only one
// whose daemon the labs on macOS use.
const ZeProfile = "ze"

// DockerHostStart is the command that starts the `ze` profile, recreating it
// when the image changed. A refusal names it on a line of its own.
const DockerHostStart = "./le setup docker-host start"

// ZeSocket answers where Colima serves the `ze` profile's Docker daemon for
// the account whose home directory is home.
func ZeSocket(home string) string {
	return filepath.Join(home, ".colima", ZeProfile, colimaSocket)
}

// Environment returns environ with the `ze` profile's socket selected as
// DOCKER_HOST, or a refusal naming DockerHostStart when that socket is absent.
//
// It returns environ unchanged on any platform other than macOS, and when
// DOCKER_HOST already carries a value. home answers the account's home
// directory, which os.UserHomeDir answers for this process, and is asked only
// when a socket must be named. socket reports whether a path is a usable
// socket, which IsSocket answers for a real filesystem.
func Environment(goos string, home func() (string, error), environ []string, socket func(string) bool) ([]string, error) {
	if goos != goosDarwin {
		return environ, nil
	}
	// os/exec resolves a duplicated variable to its LAST occurrence, so that is
	// the value the child would read and the only one worth asking about.
	named := ""
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, variable+"="); ok {
			named = value
		}
	}
	if named != "" {
		return environ, nil
	}
	homeDir, err := home()
	if err != nil {
		var b textbuf.Buffer
		return nil, errors.New(b.Str("le cannot name the `ze` Colima profile's Docker socket, because the home directory is unknown: ").
			Err(err).Str(". Set HOME to your home directory, then rerun.").String())
	}
	candidate := ZeSocket(homeDir)
	if !socket(candidate) {
		return nil, zeSocketAbsent(candidate)
	}
	var b textbuf.Buffer
	return set(environ, b.Str(variable+"=unix://").Str(candidate).String()), nil
}

// zeSocketAbsent is the refusal for a macOS run with no DOCKER_HOST and no
// `ze` profile socket: what is missing, why no other daemon is taken, and the
// command that starts the profile on a line of its own.
func zeSocketAbsent(candidate string) error {
	var b textbuf.Buffer
	return errors.New(b.Str("the Docker socket of the `ze` Colima profile, ").Str(candidate).
		Str(", is absent, and DOCKER_HOST names no daemon. On macOS the labs run on the `ze` profile,").
		Str(" which boots Ze's image and kernel; le never falls back to Colima's `default` profile or to the current Docker context,").
		Str(" which run another kernel. Start the profile with:\n  ").Str(DockerHostStart).String())
}

// Inherited returns this process's environment with the `ze` profile's socket
// selected on macOS, or Environment's refusal. It is what a command that runs
// `docker` as a child hands the child.
func Inherited() ([]string, error) {
	return Environment(runtime.GOOS, os.UserHomeDir, os.Environ(), IsSocket)
}

// IsSocket reports whether path is a socket this account can name.
func IsSocket(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

// colimaSocket is the file name of the Docker socket in a Colima profile's
// directory.
const colimaSocket = "docker.sock"

// colimaProfileConfig is the file Colima keeps in every profile's directory.
// Its presence beside the socket marks the directory as a profile's, wherever
// the Colima home is (~/.colima, $XDG_CONFIG_HOME/colima, or COLIMA_HOME).
const colimaProfileConfig = "colima.yaml"

// colimaDefaultInstance is the Lima instance of Colima's default profile,
// relative to the Colima home. Colima also forwards that profile's Docker
// socket to <home>/docker.sock (colima 29.5.2, _lima/colima/lima.yaml), so a
// socket at the home's root whose home holds this instance is the default
// profile's.
var colimaDefaultInstance = filepath.Join("_lima", "colima", "lima.yaml")

// colimaDefaultProfile is the name Colima gives the profile it starts when
// none is named.
const colimaDefaultProfile = "default"

// ColimaProfile answers the Colima profile whose VM serves endpoint, the
// address a Docker client prints for its endpoint (`docker context inspect
// --format '{{.Endpoints.docker.Host}}'`), and false when endpoint is not a
// Colima VM's socket. Colima serves each profile's daemon on
// <home>/<profile>/docker.sock beside that profile's colima.yaml, so the
// profile is that directory's name; the default profile's socket is also
// forwarded to <home>/docker.sock. exists reports whether a file exists, which
// FileExists answers for a real filesystem.
func ColimaProfile(endpoint string, exists func(string) bool) (string, bool) {
	path, unix := strings.CutPrefix(strings.TrimSpace(endpoint), "unix://")
	if !unix {
		return "", false
	}
	if filepath.Base(path) != colimaSocket {
		return "", false
	}
	dir := filepath.Dir(path)
	if exists(filepath.Join(dir, colimaProfileConfig)) {
		return filepath.Base(dir), true
	}
	if exists(filepath.Join(dir, colimaDefaultInstance)) {
		return colimaDefaultProfile, true
	}
	return "", false
}

// FileExists reports whether path names a file this account can stat.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ColimaSSH answers the argv prefix that runs a command inside the VM of the
// named Colima profile, as the VM's default user. The command follows it as
// separate words.
func ColimaSSH(profile string) []string {
	return []string{"colima", "ssh", "--profile", profile, "--"}
}

// set replaces the one variable this package names, or appends it. An empty
// DOCKER_HOST= reaches here, because Environment reads that as unset, so the
// entry is dropped rather than left to shadow the one appended after it.
func set(environ []string, value string) []string {
	name, _, _ := strings.Cut(value, "=")
	result := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if existing, _, _ := strings.Cut(entry, "="); existing != name {
			result = append(result, entry)
		}
	}
	return append(result, value)
}
