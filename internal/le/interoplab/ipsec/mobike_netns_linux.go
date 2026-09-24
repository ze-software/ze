//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- native runtime-kernel MOBIKE proof.
// Related: mobike.go -- the unchanged Docker and native scenario assertions.
// Related: mobike_netns_process_linux.go -- bounded peer process ownership.
package ipsec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	mobikeNativeScenarioTimeout = 3 * time.Minute
	mobikeNativeCommandTimeout  = 15 * time.Second
	mobikeNativeReadyTimeout    = 30 * time.Second
	mobikeNativeSessionTimeout  = 60 * time.Second
)

// RunMOBIKENetns runs both existing movement checkers against real Ze and
// strongSwan processes on one Linux kernel. The caller MUST provide the built
// Ze daemon and packaged charon/swanctl executables. Each scenario owns separate
// network namespaces and private state; teardown MUST finish before the next starts.
func RunMOBIKENetns(ctx context.Context, root, daemonPath, charonPath, swanctlPath string) (Report, int) {
	report := Report{}
	binaries, err := mobikeNativeBinaries(daemonPath, charonPath, swanctlPath)
	if err != nil {
		report.SetupError = err.Error()
		report.Code = 1
		return report, report.Code
	}
	root, err = filepath.Abs(root)
	if err != nil {
		report.SetupError = fmt.Sprintf("resolve checkout: %v", err)
		report.Code = 1
		return report, report.Code
	}
	for _, scenario := range []struct {
		name  string
		check func(context.Context, *scenarioLab) error
	}{
		{name: "mobike-initiator", check: checkMOBIKEInitiator},
		{name: "mobike-responder", check: checkMOBIKEResponder},
	} {
		result := runMOBIKENativeScenario(ctx, root, scenario.name, binaries, scenario.check)
		report.Scenarios = append(report.Scenarios, result)
		if result.Passed {
			report.Passed++
			continue
		}
		report.Failed++
		report.FailedNames = append(report.FailedNames, result.Name)
		report.Code = 1
	}
	return report, report.Code
}

func mobikeNativeBinaries(daemonPath, charonPath, swanctlPath string) (map[string]string, error) {
	if os.Geteuid() != 0 {
		return nil, errors.New("native MOBIKE requires root with network and mount namespace privileges")
	}
	paths := map[string]string{
		zePeer: daemonPath, "charon": charonPath, cmdSwanctl: swanctlPath,
		"ip": "ip", cmdPing: cmdPing, "unshare": "unshare", "mount": "mount", "sh": "sh",
	}
	for name, path := range paths {
		if path == "" {
			return nil, fmt.Errorf("native MOBIKE requires an executable path for %s", name)
		}
		resolved, err := exec.LookPath(path)
		if err != nil {
			return nil, fmt.Errorf("native MOBIKE requires %s (Alpine packages: strongswan iproute2 iputils util-linux): %w", name, err)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return nil, fmt.Errorf("resolve %s executable: %w", name, err)
		}
		paths[name] = resolved
	}
	return paths, nil
}

func runMOBIKENativeScenario(ctx context.Context, root, name string, binaries map[string]string, checker func(context.Context, *scenarioLab) error) (result interoplab.ScenarioResult) {
	result.Name = name
	if err := ctx.Err(); err != nil {
		result.Error = err.Error()
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, mobikeNativeScenarioTimeout)
	defer cancel()
	// /tmp is guest-local and root-owned; the checkout may be a 9p mount owned
	// by the host user, on which Ze correctly refuses to put its database.
	directory, err := os.MkdirTemp("/tmp", "ze-mobike-")
	if err != nil {
		result.Error = fmt.Sprintf("create native MOBIKE directory: %v", err)
		return result
	}
	lab := &mobikeNativeLab{
		directory: directory, binaries: binaries,
		peers: make(map[string]*mobikeNativePeer, 2),
	}
	defer func() {
		if result.Error != "" {
			result.Error += lab.diagnostics()
		}
		result.CleanupErrors = lab.close()
		if len(result.CleanupErrors) != 0 {
			result.Passed = false
			result.Error = strings.TrimSpace(result.Error + "\ncleanup: " + strings.Join(result.CleanupErrors, "; "))
		}
	}()
	source := interoplab.ScenarioSource{
		Name: name, Directory: filepath.Join(root, "test", "interop-ipsec", "scenarios", name),
	}
	state := &scenarioState{root: root, renderedConfig: filepath.Join(directory, "ze", "ze.conf")}
	if err := lab.prepare(ctx, root, source.Directory, state); err != nil {
		result.Error = err.Error()
		return result
	}
	check := &interoplab.CheckContext{Source: source, Lab: lab}
	if err := checker(ctx, newScenarioLab(check, mobikeNativeSessionTimeout, state)); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Passed = true
	return result
}

// mobikeNativeLab is used serially. close MUST be called after allocation, even
// after a partial prepare, to release every started process and named namespace.
type mobikeNativeLab struct {
	directory  string
	binaries   map[string]string
	peers      map[string]*mobikeNativePeer
	namespaces []string
	detached   []*mobikeNativeProcess
}

type mobikeNativePeer struct {
	namespace string
	argv      []string
	environ   []string
	process   *mobikeNativeProcess
}

var _ interoplab.CheckerLab = (*mobikeNativeLab)(nil)

func (l *mobikeNativeLab) prepare(ctx context.Context, root, source string, state *scenarioState) error {
	for _, name := range []string{"ze", "bin", "run"} {
		if err := os.Mkdir(filepath.Join(l.directory, name), 0o700); err != nil {
			return fmt.Errorf("create private %s directory: %w", name, err)
		}
	}
	if err := os.Symlink(l.binaries[zePeer], filepath.Join(l.directory, "bin", "ze")); err != nil {
		return fmt.Errorf("stage native Ze personality: %w", err)
	}
	if err := renderZeConfig(filepath.Join(source, "ze.conf"), "", state.renderedConfig); err != nil {
		return err
	}
	if err := l.writeSwanConfig(root, source); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		namespace := filepath.Base(l.directory) + "-" + peer
		// Record ownership before ip runs: cancellation may arrive after the
		// kernel created the namespace but before the command reports success.
		if _, err := os.Lstat("/run/netns/" + namespace); err == nil {
			return fmt.Errorf("native MOBIKE namespace already exists: %s", namespace)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect native MOBIKE namespace: %w", err)
		}
		l.namespaces = append(l.namespaces, namespace)
		if _, err := l.host(ctx, "netns", "add", namespace); err != nil {
			return err
		}
		l.peers[peer] = &mobikeNativePeer{namespace: namespace}
	}
	// Both veth ends are created directly in their final namespaces. A failed
	// setup cannot strand a link in the guest's SSH network namespace.
	if _, err := l.host(ctx, "link", "add", "name", containerInterface, "netns", l.peers[zePeer].namespace,
		"type", "veth", "peer", "name", containerInterface, "netns", l.peers[swanPeer].namespace); err != nil {
		return err
	}
	for _, endpoint := range []struct{ peer, address string }{
		{peer: zePeer, address: zeIP}, {peer: swanPeer, address: swanIP},
	} {
		for _, argv := range [][]string{
			{"ip", "link", "set", "lo", "up"},
			{"ip", ipObjectAddress, "add", endpoint.address + "/24", ipArgDev, containerInterface},
			{"ip", "link", "set", containerInterface, "up"},
		} {
			if _, err := l.Exec(ctx, endpoint.peer, argv, nil); err != nil {
				return err
			}
		}
	}
	return l.startPeers(ctx, source, state.renderedConfig)
}

func (l *mobikeNativeLab) writeSwanConfig(root, source string) error {
	var config strings.Builder
	config.WriteString("include /etc/strongswan.conf\n")
	for _, path := range []string{
		filepath.Join(root, "test", "interop-ipsec", swanLabConfig),
		filepath.Join(source, "strongswan.conf"),
	} {
		content, err := os.ReadFile(path) //nolint:gosec // the path comes from the checkout and the lab source directory listed above
		if err != nil {
			return fmt.Errorf("read native strongSwan settings: %w", err)
		}
		config.Write(content)
		config.WriteByte('\n')
	}
	fmt.Fprintf(&config, "charon {\n plugins {\n  vici {\n   socket = %s\n  }\n }\n}\n", l.viciURI())
	if err := os.WriteFile(filepath.Join(l.directory, "strongswan.conf"), []byte(config.String()), 0o600); err != nil {
		return fmt.Errorf("write native strongSwan settings: %w", err)
	}
	return nil
}

func (l *mobikeNativeLab) viciURI() string {
	return "unix://" + filepath.Join(l.directory, "charon.vici")
}

func (l *mobikeNativeLab) startPeers(ctx context.Context, source, zeConfig string) error {
	swan := l.peers[swanPeer]
	// charon.pid is compiled as /run/charon.pid. Only this process's private
	// mount namespace sees the bind, so neither another lab nor the host daemon
	// can collide with it. The VICI socket is also unique to this scenario.
	swan.argv = []string{l.binaries["unshare"], "--mount", "--propagation", "private",
		l.binaries["sh"], "-ec", `"$1" --bind "$2" /run; shift 2; exec "$@"`,
		"mobike-charon", l.binaries["mount"], filepath.Join(l.directory, "run"), l.binaries["charon"]}
	swan.environ = []string{"STRONGSWAN_CONF=" + filepath.Join(l.directory, "strongswan.conf")}
	if err := l.Start(ctx, swanPeer); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: mobikeNativeReadyTimeout, Interval: 200 * time.Millisecond,
		Description: "private strongSwan VICI socket",
	}, func(probe context.Context) (string, error) {
		return l.Query(probe, swanPeer, []string{cmdSwanctl, "--stats"}, nil)
	}, func(answer string) bool { return strings.Contains(answer, "uptime") })
	if err != nil {
		return err
	}
	// Load before starting Ze, matching the Docker lab. A strongSwan initiator
	// retries its first request while the responding Ze process starts.
	if _, err := l.Exec(ctx, swanPeer, []string{cmdSwanctl, "--load-all", "--file", filepath.Join(source, "swanctl.conf")}, nil); err != nil {
		return err
	}
	ze := l.peers[zePeer]
	ze.argv = []string{filepath.Join(l.directory, "bin", "ze"), "start", zeConfig}
	environ, err := zeEnvironment(source)
	if err != nil {
		return err
	}
	for _, variable := range environ {
		ze.environ = append(ze.environ, variable.Name+"="+variable.Value)
	}
	ze.environ = append(ze.environ, "ZE_CONFIG_DIR="+filepath.Dir(zeConfig))
	return l.Start(ctx, zePeer)
}

func (l *mobikeNativeLab) host(ctx context.Context, argv ...string) (interoplab.CommandResult, error) {
	return mobikeNativeCommand(ctx, append([]string{l.binaries["ip"]}, argv...), l.environment(nil), l.directory)
}

func (l *mobikeNativeLab) environment(extra []string) []string {
	// Replace rather than duplicate inherited settings. The CLI and daemon may
	// otherwise consume the host's persistent store or a different ze binary.
	environ := os.Environ()
	for _, variable := range append([]string{
		"PATH=" + filepath.Join(l.directory, "bin") + ":/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"LC_ALL=C", "STRONGSWAN_CONF=" + filepath.Join(l.directory, "strongswan.conf"),
	}, extra...) {
		name, _, _ := strings.Cut(variable, "=")
		prefix := name + "="
		kept := environ[:0]
		for _, inherited := range environ {
			if !strings.HasPrefix(inherited, prefix) {
				kept = append(kept, inherited)
			}
		}
		kept = append(kept, variable)
		environ = kept
	}
	return environ
}

func (l *mobikeNativeLab) diagnostics() string {
	var out strings.Builder
	for _, name := range []string{zePeer, swanPeer} {
		peer := l.peers[name]
		if peer == nil {
			continue
		}
		if peer.process == nil {
			continue
		}
		fmt.Fprintf(&out, "\n%s log:\n%s", name, peer.process.output.text(100))
	}
	return out.String()
}

// close MUST be called once per lab, after the checker returns. Every command
// has a fresh cleanup deadline independent of the canceled scenario context.
func (l *mobikeNativeLab) close() []string {
	var failures []string
	for _, process := range l.detached {
		if err := process.stop(3 * time.Second); err != nil {
			failures = append(failures, "detached command: "+err.Error())
		}
	}
	for _, name := range []string{zePeer, swanPeer} {
		peer := l.peers[name]
		if peer == nil {
			continue
		}
		if peer.process == nil {
			continue
		}
		if err := peer.process.stop(3 * time.Second); err != nil {
			failures = append(failures, name+": "+err.Error())
		}
	}
	for _, v := range slices.Backward(l.namespaces) {
		if _, err := os.Lstat("/run/netns/" + v); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if _, err := l.host(context.Background(), "netns", "delete", v); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if err := os.RemoveAll(l.directory); err != nil {
		failures = append(failures, "remove native MOBIKE directory: "+err.Error())
	}
	return failures
}
