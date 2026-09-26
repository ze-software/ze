// Design: docs/architecture/fleet-config.md -- a live restore of the config a hub serves to a managed client
// Related: storage_live_fixture.go -- the SSH client and artifact helpers
// Related: misc_fixture_managed_ca_trust.go -- caTrustDaemon, caTrustStart, caTrustImportConfig, caTrustPollActive

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// The router-ids the managed client's config carries: at boot, as the hub
// first serves it, and as the restored artifact carries it. Three values, so
// each step of the scenario is told apart from the one before.
const (
	restoreClientBootRouterID     = "1.1.1.1"
	restoreClientServedRouterID   = "2.2.2.2"
	restoreClientRestoredRouterID = "3.3.3.3"
	restoreClientSourceName       = "edge-yesterday.conf"
)

// storageLiveRestoreClient proves AC-8's client target with two real daemons,
// both `ze start`: a hub serving managed client edge-01, and that client. The
// client first fetches the config the hub serves. Then, over the hub's SSH,
// `request data restore path <abs> config client edge-01` writes the artifact's
// config as client-edge-01.conf, and the client's active config becomes the
// restored one within 10 seconds, well inside the 30 second heartbeat, so the
// change arrived through the config-changed push. The client's active config
// changes only after it committed the fetched config through its own reload,
// so it is the client serving the restored config. A client the hub does not
// serve is refused naming why.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func storageLiveRestoreClient(ctx context.Context, args []string) (retErr error) {
	if len(args) != 3 {
		return errors.New("usage: storage/data-restore-client-live <hub-ssh-port> <hub-plugin-port> <hub-managed-port>")
	}
	sshPort, pluginPort, managedPort := args[0], args[1], args[2]
	managed, err := strconv.Atoi(managedPort)
	if err != nil {
		return fmt.Errorf("hub-managed-port %q: %w", managedPort, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "restore-client-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := restoreClientDaemon(ctx, base, "hub", managed+1000, false)
	if err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf", restoreClientHubConfig(sshPort, pluginPort, managedPort)); err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, hub, "client-"+caTrustClientName+".conf", restoreClientConfig(managedPort, restoreClientServedRouterID)); err != nil {
		return err
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

	client, err := restoreClientDaemon(ctx, base, "client", managed+2000, true)
	if err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, client, caTrustClientName+".conf", restoreClientConfig(managedPort, restoreClientBootRouterID)); err != nil {
		return err
	}
	if err := caTrustStart(ctx, client); err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(client.command, client.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nclient output:\n%s", retErr, client.output.String())
		}
	}()
	if !caTrustPollActive(ctx, client, restoreClientServedRouterID, 300) {
		return errors.New("the client never fetched the config the hub serves")
	}

	env, err := storageLiveClient(ctx, []string{sshPort})
	if err != nil {
		return err
	}
	source := filepath.Join(base, "edge-backup.zefs")
	restored := restoreClientConfig(managedPort, restoreClientRestoredRouterID)
	if err := storageLiveSource(source, map[string][]byte{zefs.KeyFileActive.Key(restoreClientSourceName): []byte(restored)}); err != nil {
		return err
	}
	if err := storageLiveRefused(ctx, env, "request data restore path "+source+" config client edge-99", "no client entry named edge-99"); err != nil {
		return err
	}
	output, err := storageLiveRequest(ctx, env, "request data restore path "+source+" config client "+caTrustClientName)
	if err != nil {
		return err
	}
	for _, want := range []string{restoreClientSourceName, "client-" + caTrustClientName + ".conf"} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("restore answer lacks %q:\n%s", want, output)
		}
	}
	if !caTrustPollActive(ctx, client, restoreClientRestoredRouterID, 100) {
		active, _ := caTrustActiveConfig(ctx, client)
		return fmt.Errorf("the client did not take the restored config within 10s; the restore answered:\n%s\nthe client's active config:\n%s", output, active)
	}
	return storageLiveOK("data-restore-client-live")
}

// restoreClientDaemon initializes one daemon's store in base/role. A managed
// client runs `ze init --managed` and skips hub certificate verification: this
// scenario proves the push, and managed-hub-ca-trust.ci proves the trust.
func restoreClientDaemon(ctx context.Context, base, role string, bgpPort int, managedClient bool) (*caTrustDaemon, error) {
	dir := filepath.Join(base, role)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	overrides := map[string]string{
		envConfigDir:   dir,
		envNoColor:     "1",
		envTestBGPPort: strconv.Itoa(bgpPort),
	}
	name := caTrustHubName
	initArgs := []string{"init"}
	if managedClient {
		overrides["ZE_MANAGED_TLS_INSECURE"] = valueTrue
		name = caTrustClientName
		initArgs = append(initArgs, "--managed")
	}
	daemon := &caTrustDaemon{dir: dir, dbPath: filepath.Join(dir, "database"), env: miscEnvironment(overrides)}
	initInput := "admin\ntestpass\n127.0.0.1\n2222\n" + name + "\n"
	if _, err := managedRunCommand(ctx, daemon.env, dir, initInput, initArgs...); err != nil {
		return nil, err
	}
	return daemon, nil
}

// restoreClientHubConfig is the hub's own config: SSH for the request, the
// plugin transport first, and the managed listener naming the client, in the
// order caTrustHubConfig explains.
func restoreClientHubConfig(sshPort, pluginPort, managedPort string) string {
	var b textbuf.Buffer
	b.Str("system {\n    authentication {\n        user admin {\n")
	b.Str("            password \"$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO\"\n")
	b.Str("        }\n    }\n}\n")
	b.Str("environment {\n    ssh {\n        enabled true\n        server main {\n")
	b.Str("            ip 127.0.0.1;\n            port ").Str(sshPort).Str(";\n        }\n    }\n}\n")
	b.Str("plugin {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(managedPort).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	b.Str("            client ").Str(caTrustClientName).Str(" { secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("        }\n    }\n}\n")
	b.Str("bgp {\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

// restoreClientConfig is the managed client's config with the given router-id.
func restoreClientConfig(managedPort, routerID string) string {
	var b textbuf.Buffer
	b.Str("plugin {\n    hub {\n")
	b.Str("        client ").Str(caTrustClientName).Str(" { host 127.0.0.1; port ").Str(managedPort).
		Str("; secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    router-id ").Str(routerID).Str("\n}\n")
	return b.String()
}
