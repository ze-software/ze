// Design: docs/architecture/appliance/docker-host.md -- containerd and dockerd under the supervisor
// Related: supervise.go -- BuiltinUnit, which resolves these units when they start

package dockerhost

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// containerdConfig is containerd's config file. The boot seeds it so
	// Colima's provisioning, which backs the file up with `sudo cp` before it
	// writes its own, finds one (spec AC-12).
	containerdConfig = "/etc/containerd/config.toml"
	// containerdSeed is the seeded config: containerd's defaults, stated as
	// config version 2, the version Colima's own config uses.
	containerdSeed = "version = 2\n"
	// dockerDropInDir holds the drop-ins Colima writes for docker.service.
	dockerDropInDir = unitDir + "/docker.service.d"
	// containerdRoot is containerd's persistent root on a path with no
	// symlink in it (see dockerdFixedArgv); a flag, because Colima replaces
	// the config file.
	containerdRoot = "/perm/var/lib/containerd"
	// hostGatewayFlag is the one dockerd setting taken from Colima's
	// drop-in (spec D-7).
	hostGatewayFlag = "--host-gateway-ip"
)

// dockerdFixedArgv is the dockerd command line the supervisor runs whatever
// Colima's drop-in says: the drop-in's `-H fd://` assumes systemd socket
// activation, which the guest does not do, and every other daemon setting
// travels in /etc/docker/daemon.json, which dockerd reads itself (spec D-7,
// R-4). The firewall backend is dockerd's default, iptables, with the static
// legacy iptables the image ships (spec A-5).
//
// The data and exec roots are named on real paths: gokrazy makes /var a
// symlink to /perm/var and /var/run one to /run, and runc refuses a container
// rootfs whose path holds a symlink ("invalid rootfs: not an absolute path,
// or a symlink", first `docker run` on the vz guest, 2026-10-21).
var dockerdFixedArgv = []string{
	"/usr/bin/dockerd",
	"--host=unix:///var/run/docker.sock",
	"--group=docker",
	"--containerd=/run/containerd/containerd.sock",
	"--data-root=/perm/var/lib/docker",
	"--exec-root=/run/docker",
}

// DockerUnits are containerd.service and docker.service, the two services
// Colima starts, stops and restarts through systemctl. Both restart whenever
// they exit.
func DockerUnits() []BuiltinUnit {
	return []BuiltinUnit{
		FixedUnit(UnitSpec{
			Name:    "containerd.service",
			Argv:    []string{"/usr/bin/containerd", "--config", containerdConfig, "--root", containerdRoot, "--state", "/run/containerd"},
			Restart: RestartAlways,
		}),
		{Name: "docker.service", Resolve: resolveDockerd},
	}
}

// resolveDockerd answers docker.service: dockerdFixedArgv, plus the host
// gateway address Colima's drop-in names, when it names one.
func resolveDockerd(root string) (UnitSpec, error) {
	argv := slices.Clone(dockerdFixedArgv)
	gateway, err := hostGatewayFromDropIns(root)
	if err != nil {
		return UnitSpec{}, err
	}
	if gateway.IsValid() {
		argv = append(argv, hostGatewayFlag+"="+gateway.String())
	}
	return UnitSpec{Name: "docker.service", Argv: argv, Restart: RestartAlways}, nil
}

// hostGatewayFromDropIns reads every docker.service drop-in and answers the
// last --host-gateway-ip an ExecStart line gives, as systemd lets the last
// drop-in win. It answers the zero Addr when no drop-in names one, and an
// error when the value is not an IP address.
func hostGatewayFromDropIns(root string) (netip.Addr, error) {
	paths, err := filepath.Glob(guest(root, dockerDropInDir) + "/*.conf")
	if err != nil {
		return netip.Addr{}, fmt.Errorf("list %s: %w", dockerDropInDir, err)
	}
	slices.Sort(paths)
	var gateway netip.Addr
	for _, path := range paths {
		data, err := readBounded(path)
		if err != nil {
			return netip.Addr{}, err
		}
		found, err := hostGatewayFromDropIn(data)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("%s: %w", path, err)
		}
		if found.IsValid() {
			gateway = found
		}
	}
	return gateway, nil
}

// hostGatewayFromDropIn answers the --host-gateway-ip of one drop-in's
// ExecStart lines, in the `--flag=value` or `--flag value` form.
func hostGatewayFromDropIn(data []byte) (netip.Addr, error) {
	var gateway netip.Addr
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "ExecStart" {
			continue
		}
		fields := strings.Fields(value)
		for i, field := range fields {
			raw, found := strings.CutPrefix(field, hostGatewayFlag+"=")
			if !found {
				if field != hostGatewayFlag || i+1 == len(fields) {
					continue
				}
				raw = fields[i+1]
			}
			addr, err := netip.ParseAddr(raw)
			if err != nil {
				return netip.Addr{}, fmt.Errorf("%s %q is not an IP address", hostGatewayFlag, raw)
			}
			gateway = addr
		}
	}
	if err := scanner.Err(); err != nil {
		return netip.Addr{}, err
	}
	return gateway, nil
}

// seedContainerdConfig writes containerdSeed to containerdConfig unless a
// config is already there.
func seedContainerdConfig(root string) error {
	path := guest(root, containerdConfig)
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", containerdConfig, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // /etc/containerd is world-readable, as on any distribution
		return fmt.Errorf("create %s: %w", filepath.Dir(containerdConfig), err)
	}
	if err := os.WriteFile(path, []byte(containerdSeed), 0o644); err != nil { //nolint:gosec // containerd's config is world-readable, as on any distribution
		return fmt.Errorf("write %s: %w", containerdConfig, err)
	}
	return nil
}
