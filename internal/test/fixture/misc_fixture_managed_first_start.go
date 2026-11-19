// Design: docs/architecture/fleet-config.md -- the first start of a managed client that ze init --managed provisioned
// Related: misc_fixture_managed_ca_trust.go -- prepareCATrustHub, the real hub and the root it exports
// Related: misc_fixture_managed_cached_boot.go -- cachedBootStart and cachedBootWait, the real client daemon
// Related: misc_fixture_managed.go -- managedRunCommand and stopManagedProcess

package fixture

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The three clients of this scenario, the secret the hub admits them by, and
// the lines the product writes that the verdicts read.
const (
	// One name per client, because the hub admits one connection per name
	// (internal/component/plugin/server/managed_serve.go): a second client under
	// the name of one that just stopped would race the hub's release of it.
	firstStartNameRootGiven = "edge-root-given"
	firstStartNameFirstUse  = "edge-first-use"
	firstStartNameNoAnchor  = "edge-no-anchor"
	firstStartClientSecret  = "first-start-client-secret-at-least-32ch" // #nosec G101 -- a fixture secret two test daemons agree on, reachable only inside this scenario.
	firstStartPluginSecret  = "first-start-plugin-secret-at-least-32ch" // #nosec G101 -- a fixture secret for the client's own plugin transport.
	firstStartReadyName     = "daemon-started.ready"
	firstStartRootFile      = "hub-root.pem"
	// The store keys ze init --managed writes and the first start reads
	// (pkg/zefs/keys.go).
	firstStartKeyHubRoot  = "meta/managed/hub/ca"
	firstStartKeyHubTrust = "meta/managed/hub/trust"
	firstStartKeyManaged  = "meta/instance/managed"
	firstStartKeyActive   = "file/active/"
	// What keepHubRoot (cmd/ze/ze_core_start.go) prints when a first-use
	// connection kept the authority the hub presented.
	firstStartKeptNotice = "managed: trusting the certificate authority the hub presented on this first connection"
	// What cmdStartManaged prints when the first fetch fails.
	firstStartFetchFailure = "fetch config from hub"
	// The INFO line RunManagedClient (internal/component/managed/client.go)
	// writes once the running client authenticated to its hub. The first fetch
	// does not write it, so it proves the SECOND connection.
	firstStartConnectedMessage = "connected to hub"
	firstStartPollDelay        = 100 * time.Millisecond
	// The hub starts its managed listener after its plugins, which it allows 30
	// seconds (cmd/ze/hub/main.go).
	firstStartListenAttempts = 300
	// The running client connects after the daemon is ready. The hub may refuse
	// that connection while it still holds the first fetch's session under the
	// same name; the client then retries after a backoff of one second, then two.
	firstStartConnectAttempts = 300
	// A first start that cannot authenticate its hub fails at the handshake.
	// Thirty seconds leaves a loaded machine room to start the process.
	firstStartExitAttempts = 300
)

// firstStartClient is one store `ze init --managed` provisioned and what a
// `ze start` on it needs.
type firstStartClient struct {
	name      string
	dir       string
	dbPath    string
	readyPath string
	env       []string
}

// managedFirstStartDriver answers one question: does a managed client that
// `ze init --managed` provisioned, and nothing else, fetch its config from a
// real hub on a plain `ze start` and keep following that hub, authenticating it
// by the root the operator gave (--hub-ca) or by the authority the hub
// presented once (--hub-trust-first-use), and refuse the hub with neither
// (docs/architecture/fleet-config.md, "First start trust")?
//
// Method: one real hub whose own authority issues its certificate, and three
// real clients started one after the other with no --server, --name or --token
// and no ze.managed override. The config the hub stores for each names no `ca`,
// so the running client's connection is authenticated by the stored root and
// by nothing else.
func managedFirstStartDriver(ctx context.Context, args []string) (retErr error) {
	flags := flag.NewFlagSet("managed/client-first-start-from-init", flag.ContinueOnError)
	hubPluginPort := flags.Int("hub-plugin-port", 0, "hub plugin transport port")
	hubManagedPort := flags.Int("hub-managed-port", 0, "hub managed-client listen port")
	clientPluginPort := flags.Int("client-plugin-port", 0, "the clients' own plugin transport port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *hubPluginPort == 0 {
		return errors.New("hub-plugin-port is required")
	}
	if *hubManagedPort == 0 {
		return errors.New("hub-managed-port is required")
	}
	if *clientPluginPort == 0 {
		return errors.New("client-plugin-port is required")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "managed-first-start-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	// The hub runs once to generate and export its root, and is stopped again
	// (prepareCATrustHub). Its store is then given the three clients and the
	// config it serves each, which an offline import can only do while no
	// daemon holds the store.
	hub, err := prepareCATrustHub(ctx, base, *hubPluginPort, *hubManagedPort)
	if err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf",
		firstStartHubConfig(*hubPluginPort, *hubManagedPort)); err != nil {
		return err
	}
	served := make(map[string]string, 3)
	for _, name := range []string{firstStartNameRootGiven, firstStartNameFirstUse, firstStartNameNoAnchor} {
		served[name] = firstStartServedConfig(name, *clientPluginPort, *hubManagedPort)
		if err := caTrustImportConfig(ctx, hub, "client-"+name+".conf", served[name]); err != nil {
			return err
		}
	}
	if err := caTrustStart(ctx, hub); err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(hub.command, hub.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nhub output:\n%s", retErr, hub.output.String())
		}
	}()

	hubAddress := "127.0.0.1:" + strconv.Itoa(*hubManagedPort)
	if !firstStartHubListens(ctx, hub, hubAddress) {
		return fmt.Errorf("the hub never listened for managed clients on %s", hubAddress)
	}

	rootPath := filepath.Join(base, firstStartRootFile)
	if err := os.WriteFile(rootPath, hub.rootPEM, 0o600); err != nil {
		return err
	}

	if err := firstStartRootGiven(ctx, base, *clientPluginPort, hubAddress, rootPath, hub.rootPEM,
		served[firstStartNameRootGiven]); err != nil {
		return fmt.Errorf("--hub-ca: %w", err)
	}
	if err := firstStartFirstUse(ctx, base, *clientPluginPort, hubAddress, hub.rootPEM,
		served[firstStartNameFirstUse]); err != nil {
		return fmt.Errorf("--hub-trust-first-use: %w", err)
	}
	// Last, so the hub it refuses is one that served two clients a moment ago.
	if err := firstStartNoAnchor(ctx, base, *clientPluginPort, hubAddress); err != nil {
		return fmt.Errorf("neither --hub-ca nor --hub-trust-first-use: %w", err)
	}

	fmt.Fprintln(os.Stderr, "OK: a managed client provisioned by ze init --managed alone fetched its config and kept following its hub, "+
		"with the hub root given and with the root kept on first use, and refused the hub with neither")
	return nil
}

// firstStartRootGiven is the client whose operator carried the hub root to it.
// It MUST follow the hub and MUST NOT report a first-use trust decision.
func firstStartRootGiven(ctx context.Context, base string, pluginPort int, hubAddress, rootPath string, hubRoot []byte, served string) error {
	client, err := firstStartProvision(ctx, base, firstStartNameRootGiven, pluginPort, hubAddress, "--hub-ca", rootPath)
	if err != nil {
		return err
	}
	return firstStartFollowsHub(ctx, client, served, hubAddress, func(output string) error {
		if strings.Contains(output, firstStartKeptNotice) {
			return errors.New("the client reported trusting the hub's authority on first use, though it was given the hub root")
		}
		return firstStartRequireStoredRoot(ctx, client, hubRoot)
	})
}

// firstStartFirstUse is the client told to keep the authority the hub presents.
// Its store holds no root before the start and the hub's root after it.
func firstStartFirstUse(ctx context.Context, base string, pluginPort int, hubAddress string, hubRoot []byte, served string) error {
	client, err := firstStartProvision(ctx, base, firstStartNameFirstUse, pluginPort, hubAddress, "--hub-trust-first-use")
	if err != nil {
		return err
	}
	keys, err := firstStartStoreKeys(ctx, client)
	if err != nil {
		return err
	}
	if !strings.Contains(keys, firstStartKeyHubTrust) {
		return fmt.Errorf("ze init --managed --hub-trust-first-use did not store %s; the store holds:\n%s", firstStartKeyHubTrust, keys)
	}
	if strings.Contains(keys, firstStartKeyHubRoot) {
		return fmt.Errorf("the store holds %s before the first start, so first use would prove nothing", firstStartKeyHubRoot)
	}
	return firstStartFollowsHub(ctx, client, served, hubAddress, func(output string) error {
		if !strings.Contains(output, firstStartKeptNotice) {
			return errors.New("the client did not report the certificate authority it trusted on first use")
		}
		return firstStartRequireStoredRoot(ctx, client, hubRoot)
	})
}

// firstStartNoAnchor is the control: the same provisioning with no trust
// anchor. The system pool does not know the hub's authority, so the start MUST
// fail at the handshake and MUST NOT cache a config. Without it, a client that
// verified nothing would pass both cases above.
func firstStartNoAnchor(ctx context.Context, base string, pluginPort int, hubAddress string) (retErr error) {
	client, err := firstStartProvision(ctx, base, firstStartNameNoAnchor, pluginPort, hubAddress)
	if err != nil {
		return err
	}
	daemon, err := cachedBootStart(ctx, client.dir, client.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nze output:\n%s", retErr, daemon.output.String())
		}
	}()

	exited := Poll(ctx, firstStartExitAttempts, firstStartPollDelay, func() bool {
		return firstStartExited(daemon.done)
	})
	if !exited {
		return errors.New("ze start kept running against a hub it had no trust anchor for")
	}
	// The reap result goes back on the channel: stopManagedProcess reads the
	// one value caTrustReap sends.
	reaped := <-daemon.done
	daemon.done <- reaped
	if reaped == nil {
		return errors.New("ze start exited zero against a hub it had no trust anchor for")
	}
	if !firstStartRefusedCertificate(daemon.output.String()) {
		return errors.New("ze start failed without naming the hub's certificate as the reason, so the refusal is unexplained")
	}
	if _, statErr := os.Stat(client.readyPath); statErr == nil {
		return errors.New("the daemon published its ready file against a hub it had no trust anchor for")
	}

	keys, err := firstStartStoreKeys(ctx, client)
	if err != nil {
		return err
	}
	// The listing is proven to be one by a key ze init always writes.
	if !strings.Contains(keys, firstStartKeyManaged) {
		return fmt.Errorf("the store listing does not hold %s, so it cannot answer whether a config was cached:\n%s", firstStartKeyManaged, keys)
	}
	if strings.Contains(keys, firstStartKeyActive) {
		return fmt.Errorf("the client cached a config from a hub it could not authenticate:\n%s", keys)
	}
	return nil
}

// firstStartProvision runs `ze init --managed` with trustArgs on a new store
// and feeds it the seven lines an operator types: user, password, SSH host and
// port, the instance name, the hub address and the hub token.
func firstStartProvision(ctx context.Context, base, name string, pluginPort int, hubAddress string, trustArgs ...string) (*firstStartClient, error) {
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	client := &firstStartClient{
		name:      name,
		dir:       dir,
		dbPath:    filepath.Join(dir, "database"),
		readyPath: filepath.Join(dir, firstStartReadyName),
	}
	// Everything the start connects with MUST come from the store. The three
	// troubleshooting overrides replace the stored values when set
	// (readManagedBootstrap, cmd/ze/ze_core_start.go), and
	// ze.managed.tls.insecure would make every case pass against any
	// certificate, so all four leave the environment under every spelling.
	inherited := withoutEnv(os.Environ(), "ze.managed.server", "ze.managed.name", "ze.managed.token", "ze.managed.tls.insecure")
	client.env = childEnvironment(daemonEnvironment(inherited, dir, daemonStoreDisk), map[string]string{
		envNoColor:     "1",
		envReadyFile:   client.readyPath,
		envTestBGPPort: strconv.Itoa(pluginPort + 1000),
		// Bounds the first fetch and each later attempt. Ten seconds leaves a
		// loaded machine room for a loopback handshake.
		envManagedConnectTimeout: "10s",
		// The verdict reads this subsystem's INFO line, so its level is stated
		// here and not left to what the runner's environment carries.
		envLogHubManaged: logLevelInfo,
	})

	input := "admin\nsecret123\n127.0.0.1\n2222\n" + name + "\n" + hubAddress + "\n" + firstStartClientSecret + "\n"
	initArgs := append([]string{"init", "--managed"}, trustArgs...)
	if _, err := managedRunCommand(ctx, client.env, dir, input, initArgs...); err != nil {
		return nil, err
	}
	return client, nil
}

// firstStartFollowsHub starts the client with a plain `ze start` and requires
// three states in order: the daemon is ready, its store caches the config the
// hub serves it, and its running managed client authenticated to the hub a
// second time. verdict then judges what is particular to the case, on the
// daemon's output so far and while the daemon still runs.
func firstStartFollowsHub(ctx context.Context, client *firstStartClient, served, hubAddress string, verdict func(output string) error) (retErr error) {
	daemon, err := cachedBootStart(ctx, client.dir, client.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nze output:\n%s", retErr, daemon.output.String())
		}
	}()

	ready := cachedBootWait(ctx, daemon, cachedBootReadyAttempts, func() bool {
		_, statErr := os.Stat(client.readyPath)
		return statErr == nil
	})
	if !ready {
		return errors.New("the managed client did not become ready on its first start: it published no ready file")
	}

	// The first start caches the config before it boots on it, so a ready
	// daemon holds it already.
	cached, err := firstStartStoreKey(ctx, client, firstStartKeyActive+client.name+".conf")
	if err != nil {
		return err
	}
	if !strings.Contains(cached, strings.TrimSpace(served)) {
		return fmt.Errorf("the client's cached config is not the one the hub serves it.\ncached:\n%s\nserved:\n%s", cached, served)
	}

	connected := cachedBootWait(ctx, daemon, firstStartConnectAttempts, func() bool {
		return firstStartConnected(daemon.output.String(), hubAddress)
	})
	output := daemon.output.String()
	if refusal := firstStartCertificateRefusal(output); refusal != "" {
		return fmt.Errorf("the running client refused its hub's certificate after the first fetch: %s", refusal)
	}
	if !connected {
		return fmt.Errorf("the running client never authenticated to its hub at %s after the first fetch, or the daemon stopped", hubAddress)
	}
	return verdict(output)
}

// firstStartRequireStoredRoot requires the client's store to hold the hub's
// root. The certificates are compared, not their PEM text, so a difference in
// line wrapping is not a difference in the root.
func firstStartRequireStoredRoot(ctx context.Context, client *firstStartClient, hubRoot []byte) error {
	stored, err := firstStartStoreKey(ctx, client, firstStartKeyHubRoot)
	if err != nil {
		return err
	}
	storedBlock, _ := pem.Decode([]byte(stored))
	if storedBlock == nil {
		return fmt.Errorf("the store key %s holds no PEM certificate: %q", firstStartKeyHubRoot, stored)
	}
	hubBlock, _ := pem.Decode(hubRoot)
	if hubBlock == nil {
		return fmt.Errorf("the root the hub exported is not PEM: %q", hubRoot)
	}
	if !bytes.Equal(storedBlock.Bytes, hubBlock.Bytes) {
		return fmt.Errorf("the store key %s does not hold the hub's root.\nstored:\n%s\nhub:\n%s", firstStartKeyHubRoot, stored, hubRoot)
	}
	return nil
}

// firstStartStoreKey reads one key of the client's store through `ze data`,
// which reads without a second writable handle while the daemon runs. It
// answers stdout alone: a warning the command logs on stderr is not the key.
func firstStartStoreKey(ctx context.Context, client *firstStartClient, key string) (string, error) {
	stdout, stderr, code, err := rawCommandStreams(ctx, client.dir, client.env,
		"ze", "data", "--path", client.dbPath, "cat", key)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("ze data cat %s exited %d: %s", key, code, stderr)
	}
	return stdout, nil
}

// firstStartStoreKeys lists the keys of the client's store, from stdout alone.
func firstStartStoreKeys(ctx context.Context, client *firstStartClient) (string, error) {
	stdout, stderr, code, err := rawCommandStreams(ctx, client.dir, client.env,
		"ze", "data", "--path", client.dbPath, "list")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("ze data list exited %d: %s", code, stderr)
	}
	return stdout, nil
}

// firstStartHubListens reports whether the hub accepted a TCP connection on its
// managed address while it was still running. The ready file is no barrier for
// this: the hub starts its managed listener after writing it
// (cmd/ze/hub/main.go). The probe closes without a handshake, which the hub
// logs and survives.
func firstStartHubListens(ctx context.Context, hub *caTrustDaemon, address string) bool {
	dialer := net.Dialer{Timeout: time.Second}
	exited := false
	listening := Poll(ctx, firstStartListenAttempts, firstStartPollDelay, func() bool {
		if firstStartExited(hub.done) {
			exited = true
			return true
		}
		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			return false
		}
		conn.Close() //nolint:errcheck,gosec // a probe that carried nothing
		return true
	})
	if exited {
		return false
	}
	return listening
}

// firstStartExited reports whether the daemon behind done was reaped. The reap
// result goes back on the channel, because stopManagedProcess reads the one
// value caTrustReap sends.
func firstStartExited(done chan error) bool {
	select {
	case reaped := <-done:
		done <- reaped
		return true
	default:
		return false
	}
}

// firstStartConnected reports whether output holds the line the running
// managed client writes once it authenticated to the hub at hubAddress.
func firstStartConnected(output, hubAddress string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, firstStartConnectedMessage) {
			continue
		}
		if !strings.Contains(line, cachedBootSubsystem) {
			continue
		}
		if strings.Contains(line, hubAddress) {
			return true
		}
	}
	return false
}

// firstStartCertificateRefusal returns the first line of output in which the
// running managed client lost its hub over the hub's certificate, and "" when
// there is none. A connection lost for another reason is not one: the hub
// admits one connection per name and may still hold the first fetch's session
// when the running client arrives, which the client retries.
func firstStartCertificateRefusal(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, cachedBootReconnectMessage) {
			continue
		}
		if !strings.Contains(line, cachedBootSubsystem) {
			continue
		}
		if strings.Contains(line, "certificate") {
			return line
		}
		if strings.Contains(line, "x509") {
			return line
		}
	}
	return ""
}

// firstStartRefusedCertificate reports whether output holds the line with
// which a first start says its fetch failed over the hub's certificate. A hub
// that was merely unreachable fails the fetch too, and names no certificate.
func firstStartRefusedCertificate(output string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, firstStartFetchFailure) {
			continue
		}
		if strings.Contains(line, "certificate") {
			return true
		}
	}
	return false
}

// firstStartHubConfig is the hub's config for the serving run: the plugin
// transport first and the managed listener second, as caTrustHubConfig
// explains, with one entry per client. The managed block names no
// `certificate`, so the hub serves a leaf its own authority issued and presents
// that authority after it.
func firstStartHubConfig(pluginPort, managedPort int) string {
	var b textbuf.Buffer
	b.Str("plugin {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(strconv.Itoa(pluginPort)).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(strconv.Itoa(managedPort)).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	for _, name := range []string{firstStartNameRootGiven, firstStartNameFirstUse, firstStartNameNoAnchor} {
		b.Str("            client ").Str(name).Str(" { secret \"").Str(firstStartClientSecret).Str("\"; }\n")
	}
	b.Str("        }\n    }\n}\n")
	b.Str("bgp {\n    session { asn { local 65000; } }\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

// firstStartServedConfig is the config the hub stores for one client: the
// client's own plugin transport, the hub it keeps following, and a BGP section
// with no peer. Its hub client block names no `ca`, so the only thing that can
// authenticate the hub on the running client's connection is the root the
// client's store holds.
func firstStartServedConfig(name string, pluginPort, hubPort int) string {
	var b textbuf.Buffer
	b.Str("plugin {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(strconv.Itoa(pluginPort)).
		Str("; secret \"").Str(firstStartPluginSecret).Str("\"; }\n")
	b.Str("        client ").Str(name).Str(" { host 127.0.0.1; port ").
		Str(strconv.Itoa(hubPort)).Str("; secret \"").Str(firstStartClientSecret).Str("\"; }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    router-id ").Str(caTrustServedRouterID).Str("\n    session { asn { local 65000; } }\n}\n")
	return b.String()
}
