// Design: docs/architecture/core-design.md -- le's composition, one import per tool

// Package dockerhost names the Docker daemon socket a development command must
// talk to when nothing else can name one.
//
// Colima does not serve /var/run/docker.sock, which is where a Docker client
// with no DOCKER_HOST and no context looks. It serves
// ~/.colima/default/docker.sock and writes a context that points there, and
// that context is what usually connects the two. A context can also be absent
// or broken: on the machine this package was written for, ~/.docker/config.json
// names `colima` while its metadata lives under a different XDG root, so the
// client falls back to /var/run/docker.sock and every `docker` call fails with
// "check if the path is correct and if the daemon is running".
//
// DOCKER_HOST outranks a context, so naming the socket unconditionally would
// redirect an operator whose context works today. Environment therefore selects
// only when the client's own default endpoint is absent.
package dockerhost

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// goosDarwin is the only platform Colima serves.
const goosDarwin = "darwin"

// variable is the name a Docker client reads before it consults its context.
const variable = "DOCKER_HOST"

// defaultSocket is where a Docker client looks when nothing names an endpoint.
// Docker Desktop and a Linux daemon both serve it; Colima does not.
const defaultSocket = "/var/run/docker.sock"

// Environment returns environ with the Colima socket selected as DOCKER_HOST.
//
// It returns environ unchanged on any platform other than macOS, when
// DOCKER_HOST already carries a value, when the client's default endpoint
// exists, and when the Colima socket does not. socket reports whether a path is
// a usable socket, which IsSocket answers for a real filesystem.
func Environment(goos, home string, environ []string, socket func(string) bool) []string {
	if goos != goosDarwin {
		return environ
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
		return environ
	}
	// A reachable default endpoint means the client can answer for itself, and
	// naming a socket over it would move an operator's daemon under them.
	if socket(defaultSocket) {
		return environ
	}
	candidate := filepath.Join(home, ".colima", "default", "docker.sock")
	if !socket(candidate) {
		return environ
	}
	var b textbuf.Buffer
	return set(environ, b.Str(variable+"=unix://").Str(candidate).String())
}

// Inherited returns this process's environment with the Colima socket selected.
// It is what a command that runs `docker` as a child hands the child.
func Inherited() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.Environ()
	}
	return Environment(runtime.GOOS, home, os.Environ(), IsSocket)
}

// IsSocket reports whether path is a socket this account can name.
func IsSocket(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
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
