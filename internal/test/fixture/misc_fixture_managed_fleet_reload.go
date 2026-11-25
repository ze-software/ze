// Design: docs/architecture/fleet-config.md -- live managed reload, certificate rotation and session refusal doctor checks
// Related: misc_fixture_managed_fleet_registry.go -- fleetRegistryRow and registryRows
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun, fleetHoldImportBeside
// Related: register_managed_fleet_registry.go -- registers all fleet-1 fixture drivers

package fixture

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
)

const (
	fleetReloadSecondClient = "edge-02"
	fleetReloadSecondSecret = "edge-02-ca-trust-secret-at-least-32char" // #nosec G101 -- test fixture secret
	fleetReloadKeepClient   = "edge-keep"
	fleetReloadKeepSecret   = "edge-keep-secret-that-is-at-least-32ch" // #nosec G101 -- test fixture secret
)

// hubConfigNoManagedClients declares only the local plugin server and SSH
// listener, with no managed client serving block.
func hubConfigNoManagedClients(sshPort, pluginPort, _ string) string {
	var b textbuf.Buffer
	b.Str(restoreClientSSHListener(sshPort))
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("    }\n}\n")
	return b.String()
}

// managedFleetDevicesReloadAddDriver validates AC-8 and AC-26: a hub booted
// with no managed client gains one by live reload, lists it as never connected,
// then lists it connected once the client authenticates.
func managedFleetDevicesReloadAddDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-devices-reload-add", args, false, hubConfigNoManagedClients,
		func(ctx context.Context, s *fleetHoldScenario) error {
			reloaded := restoreClientHubConfig(args[0], args[1], args[2])
			if err := fleetHoldImportBeside(ctx, s.hubImporter, s.hub, caTrustHubName+".conf", []byte(reloaded)); err != nil {
				return fmt.Errorf("reload hub adding %s: %w", caTrustClientName, err)
			}
			var rows []fleetRegistryRow
			var readErr error
			listed := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				rows, readErr = s.registryRows(ctx, "show fleet devices")
				if readErr != nil {
					return false
				}
				if len(rows) != 1 {
					return false
				}
				r := rows[0]
				return r.Name == caTrustClientName && r.Connected != nil && !*r.Connected && !r.EverConnected
			})
			if !listed {
				return fmt.Errorf("after reload adding %s, show fleet devices did not list it never-connected: %+v (err=%v)",
					caTrustClientName, rows, readErr)
			}
			if err := s.withDevice(ctx, func(_ *cachedBootDaemon) error {
				_, err := s.registryRow(ctx, true)
				return err
			}); err != nil {
				return err
			}
			return storageLiveOK("reload added managed client and listed it connected")
		})
}

// managedFleetDevicesReloadRemoveDriver validates AC-9: removing a connected
// client by reload closes its session promptly, keeps its stored history marked
// removed with removed-at, and refuses its reconnect under doctor-managed-client-rejected.
func managedFleetDevicesReloadRemoveDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-devices-reload-remove", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		return s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			online, err := s.registryRow(ctx, true)
			if err != nil {
				return err
			}
			// Keep the managed listener bound for another declared client so edge-01's
			// reconnect hits the live listener and is rejected as an unknown name.
			keepOnly := "            client " + fleetReloadKeepClient + " { secret \"" + fleetReloadKeepSecret + "\"; }\n"
			reloaded := restoreClientHubConfigWithClients(args[0], args[1], args[2], keepOnly)
			if err := fleetHoldImportBeside(ctx, s.hubImporter, s.hub, caTrustHubName+".conf", []byte(reloaded)); err != nil {
				return fmt.Errorf("reload hub removing %s: %w", caTrustClientName, err)
			}
			const detailCmd = "show fleet device name " + caTrustClientName + " | json compact"
			var detailAnswer string
			removed := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
				var reqErr error
				detailAnswer, reqErr = storageLiveRequest(ctx, s.hubOperator, detailCmd)
				if reqErr != nil {
					return false
				}
				var detail struct {
					fleetRegistryRow
					RemovedAt string `json:"removed-at"`
				}
				if fleetDivergenceDecode(detailAnswer, &detail) != nil {
					return false
				}
				return detail.Name == caTrustClientName && detail.Connected != nil && !*detail.Connected &&
					detail.Removed != nil && *detail.Removed && registryTime(detail.RemovedAt) &&
					detail.EverConnected && detail.ConnectedSince == online.ConnectedSince && detail.SourceAddress != ""
			})
			if !removed {
				return fmt.Errorf("after reload removing %s, detail never showed disconnected/removed with retained history: %s",
					caTrustClientName, detailAnswer)
			}
			// Wait for the still-running device's reconnect attempt to be rejected and recorded in hub doctor.
			var doctorAnswer string
			rejected := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
				doctorAnswer, _ = storageLiveRequest(ctx, s.hubOperator, "show doctor") //nolint:errcheck // doctor warnings may exit non-zero
				return strings.Contains(doctorAnswer, fleetHoldRejectedCode) && strings.Contains(doctorAnswer, caTrustClientName)
			})
			if !rejected {
				return fmt.Errorf("removed client reconnect was not recorded under %s in hub doctor:\n%s",
					fleetHoldRejectedCode, doctorAnswer)
			}
			return storageLiveOK("reload removed connected managed client, kept history and refused reconnect")
		})
	})
}

// exportHubLocalCAPEM reads the hub's local CA root through the operator
// command `show pki local-ca pem | json compact` and verifies it decodes as PEM.
func exportHubLocalCAPEM(ctx context.Context, hubOp []string) (string, error) {
	raw, err := storageLiveRequest(ctx, hubOp, "show pki local-ca pem | json compact")
	if err != nil {
		return "", fmt.Errorf("show pki local-ca pem: %w (%s)", err, raw)
	}
	var doc struct {
		PEM string `json:"pem"`
	}
	if err := fleetDivergenceDecode(raw, &doc); err != nil {
		return "", fmt.Errorf("decode show pki local-ca pem: %w (%s)", err, raw)
	}
	block, _ := pem.Decode([]byte(doc.PEM))
	if block == nil {
		return "", fmt.Errorf("show pki local-ca pem returned invalid PEM: %q", doc.PEM)
	}
	return doc.PEM, nil
}

// verifyingClientDaemon initializes a managed client daemon WITHOUT
// ZE_MANAGED_TLS_INSECURE so every TLS handshake verifies the hub certificate.
func verifyingClientDaemon(ctx context.Context, base, role, name string) (*caTrustDaemon, error) {
	dir := filepath.Join(base, role)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	daemon := &caTrustDaemon{
		dir:    dir,
		dbPath: filepath.Join(dir, "database"),
		env: miscEnvironment(map[string]string{
			envConfigDir:     dir,
			envNoColor:       "1",
			envLogHubManaged: logLevelInfo,
		}),
	}
	initInput := "admin\ntestpass\n127.0.0.1\n2222\n" + name + "\n"
	if _, err := managedRunCommand(ctx, daemon.env, dir, initInput, argInit, "--managed"); err != nil {
		return nil, err
	}
	return daemon, nil
}

// verifyingClientConfig builds a managed client configuration anchored on a
// pki ca entry holding anchorPEM, dialing hubHost:managedPort.
func verifyingClientConfig(extra, name, secret, hubHost, managedPort, routerID, anchorPEM string) string {
	var b textbuf.Buffer
	b.Str(extra)
	b.Str("pki {\n    ca ").Str(caTrustAnchorName).Str(" {\n        certificate \"")
	b.Str(strings.TrimSpace(anchorPEM)).Str("\";\n    }\n}\n")
	b.Str("bus {\n    hub {\n")
	b.Str("        client ").Str(name).Str(" {\n")
	b.Str("            host ").Str(hubHost).Str(";\n")
	b.Str("            port ").Str(managedPort).Str(";\n")
	b.Str("            secret \"").Str(secret).Str("\";\n")
	b.Str("            ca ").Str(caTrustAnchorName).Str(";\n")
	b.Str("        }\n    }\n}\n")
	b.Str("bgp {\n    router-id ").Str(routerID).Str("\n    session { asn { local 65000; } }\n}\n")
	return b.String()
}

// hubConfigServingHost builds a hub configuration serving on listenHost:managedPort
// with optional pkiBlock and optional certificateName.
func hubConfigServingHost(sshPort, pluginPort, listenHost, managedPort, pkiBlock, certificateName, clients string) string {
	var b textbuf.Buffer
	b.Str(pkiBlock)
	b.Str(restoreClientSSHListener(sshPort))
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server central {\n")
	b.Str("            ip ").Str(listenHost).Str(";\n")
	b.Str("            port ").Str(managedPort).Str(";\n")
	b.Str("            secret \"").Str(caTrustHubSecret).Str("\";\n")
	if certificateName != "" {
		b.Str("            certificate ").Str(certificateName).Str(";\n")
	}
	b.Str(clients)
	b.Str("        }\n    }\n}\n")
	return b.String()
}

// generateRotatedPKI creates an independent CA and server certificate named
// certName valid for 127.0.0.1 and localhost, returning the pki config block
// and the CA root PEM for clients that trust only this CA.
func generateRotatedPKI(certName string) (pkiBlock, caPEM string, err error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(101),
		Subject:               pkix.Name{CommonName: certName + "-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return "", "", err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(102),
		Subject:      pkix.Name{CommonName: certName},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		return "", "", err
	}
	caPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caB64 := base64.StdEncoding.EncodeToString(caDER)
	leafB64 := base64.StdEncoding.EncodeToString(leafDER)
	keyB64 := base64.StdEncoding.EncodeToString(keyDER)

	var b textbuf.Buffer
	b.Str("pki {\n")
	b.Str("    ca ").Str(certName).Str("-root {\n        certificate ").Str(caB64).Str(";\n    }\n")
	b.Str("    certificate ").Str(certName).Str(" {\n")
	b.Str("        certificate ").Str(leafB64).Str(";\n")
	b.Str("        private {\n            key ").Str(keyB64).Str(";\n        }\n")
	b.Str("    }\n}\n")
	return b.String(), string(caPEMBytes), nil
}

// managedFleetReloadListenerVerifiedDriver validates AC-35 and A-10: a hub
// started with no managed client entry gains a serving block on 127.0.0.2 by
// reload, issuing a leaf that covers 127.0.0.2; a verifying client anchored on
// the hub's local CA connects to 127.0.0.2 and is listed connected.
func managedFleetReloadListenerVerifiedDriver(ctx context.Context, args []string) (retErr error) {
	if len(args) != 3 {
		return errors.New("usage: managed/fleet-reload-listener-verified <hub-ssh-port> <hub-plugin-port> <hub-managed-port>")
	}
	sshPort, pluginPort, managedPort := args[0], args[1], args[2]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "fleet-reload-verified-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := restoreClientDaemon(ctx, base, "hub", false)
	if err != nil {
		return err
	}
	hub.env = childEnvironment(hub.env, map[string]string{"ze.log.hub": logLevelInfo})
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf", hubConfigNoManagedClients(sshPort, pluginPort, managedPort)); err != nil {
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
	hubOp, err := storageLiveClient(ctx, []string{sshPort})
	if err != nil {
		return err
	}
	hubImporter := fleetHoldOperatorEnv(hub.env, sshPort)

	// Reload adding a serving block on 127.0.0.2 with no certificate name.
	clients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret + "\"; }\n"
	reloaded := hubConfigServingHost(sshPort, pluginPort, "127.0.0.2", managedPort, "", "", clients)
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, caTrustHubName+".conf", []byte(reloaded)); err != nil {
		return fmt.Errorf("reload hub with 127.0.0.2 managed listener: %w", err)
	}
	// Export the hub's local CA root (now initialized by the reload).
	hubRootPEM, err := exportHubLocalCAPEM(ctx, hubOp)
	if err != nil {
		return err
	}

	// Provision client-edge-01.conf on the running hub, then start a verifying client dialing 127.0.0.2.
	servedCfg := verifyingClientConfig("", caTrustClientName, caTrustClientSecret, "127.0.0.2", managedPort, restoreClientServedRouterID, hubRootPEM)
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, "client-"+caTrustClientName+".conf", []byte(servedCfg)); err != nil {
		return fmt.Errorf("import client config on hub: %w", err)
	}
	device, err := verifyingClientDaemon(ctx, base, "device", caTrustClientName)
	if err != nil {
		return err
	}
	bootCfg := verifyingClientConfig("", caTrustClientName, caTrustClientSecret, "127.0.0.2", managedPort, restoreClientBootRouterID, hubRootPEM)
	if err := caTrustImportConfig(ctx, device, caTrustClientName+".conf", bootCfg); err != nil {
		return err
	}
	daemon, err := cachedBootStart(ctx, device.dir, device.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\ndevice output:\n%s", retErr, daemon.output.String())
		}
	}()

	var answer string
	ok := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		var reqErr error
		answer, reqErr = storageLiveRequest(ctx, hubOp, "show fleet devices | raw")
		if reqErr != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(answer, &rows) != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		return rows[0].Name == caTrustClientName && rows[0].Connected != nil && *rows[0].Connected && rows[0].ConfigState == "in-sync"
	})
	if !ok {
		return fmt.Errorf("verifying client never connected to reloaded 127.0.0.2 listener: %s", answer)
	}
	return storageLiveOK("reload issued leaf covering 127.0.0.2 and verifying client connected")
}

// managedFleetReloadCertificateRotatedDriver validates AC-38 and A-12: rotating
// bus hub server central certificate to fleet-hub admits edge-02 (anchored only
// on fleet-hub's CA, refused before reload) while keeping edge-01 connected
// with connected-since unchanged.
func managedFleetReloadCertificateRotatedDriver(ctx context.Context, args []string) (retErr error) {
	if len(args) != 3 {
		return errors.New("usage: managed/fleet-reload-certificate-rotated <hub-ssh-port> <hub-plugin-port> <hub-managed-port>")
	}
	sshPort, pluginPort, managedPort := args[0], args[1], args[2]
	pkiBlock, rotatedCAPEM, err := generateRotatedPKI("fleet-hub")
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "fleet-cert-rotate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := restoreClientDaemon(ctx, base, "hub", false)
	if err != nil {
		return err
	}
	hub.env = childEnvironment(hub.env, map[string]string{"ze.log.hub": logLevelInfo})
	clients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret + "\"; }\n" +
		"            client " + fleetReloadSecondClient + " { secret \"" + fleetReloadSecondSecret + "\"; }\n"
	initialHub := hubConfigServingHost(sshPort, pluginPort, "127.0.0.1", managedPort, pkiBlock, "", clients)
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf", initialHub); err != nil {
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
	hubOp, err := storageLiveClient(ctx, []string{sshPort})
	if err != nil {
		return err
	}
	hubImporter := fleetHoldOperatorEnv(hub.env, sshPort)
	hubRootPEM, err := exportHubLocalCAPEM(ctx, hubOp)
	if err != nil {
		return err
	}

	// Provision both client configs on the hub.
	cfg1Served := verifyingClientConfig("", caTrustClientName, caTrustClientSecret, "127.0.0.1", managedPort, restoreClientServedRouterID, hubRootPEM)
	cfg2Served := verifyingClientConfig("", fleetReloadSecondClient, fleetReloadSecondSecret, "127.0.0.1", managedPort, restoreClientServedRouterID, rotatedCAPEM)
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, "client-"+caTrustClientName+".conf", []byte(cfg1Served)); err != nil {
		return err
	}
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, "client-"+fleetReloadSecondClient+".conf", []byte(cfg2Served)); err != nil {
		return err
	}

	// Start verifying edge-01 anchored on hub local CA.
	dev1, err := verifyingClientDaemon(ctx, base, "dev1", caTrustClientName)
	if err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, dev1, caTrustClientName+".conf",
		verifyingClientConfig("", caTrustClientName, caTrustClientSecret, "127.0.0.1", managedPort, restoreClientBootRouterID, hubRootPEM)); err != nil {
		return err
	}
	d1, err := cachedBootStart(ctx, dev1.dir, dev1.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(d1.command, d1.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nedge-01 output:\n%s", retErr, d1.output.String())
		}
	}()

	var edge1Since string
	connected1 := cachedBootWait(ctx, d1, fleetHoldDoctorAttempts, func() bool {
		ans, reqErr := storageLiveRequest(ctx, hubOp, "show fleet devices | raw")
		if reqErr != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(ans, &rows) != nil {
			return false
		}
		for _, r := range rows {
			if r.Name == caTrustClientName && r.Connected != nil && *r.Connected && r.ConfigState == "in-sync" && registryTime(r.ConnectedSince) {
				edge1Since = r.ConnectedSince
				return true
			}
		}
		return false
	})
	if !connected1 {
		return errors.New("edge-01 never connected before certificate rotation")
	}

	// Start verifying edge-02 anchored ONLY on rotatedCAPEM; before reload it MUST refuse the hub's issued leaf.
	dev2, err := verifyingClientDaemon(ctx, base, "dev2", fleetReloadSecondClient)
	if err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, dev2, fleetReloadSecondClient+".conf",
		verifyingClientConfig("", fleetReloadSecondClient, fleetReloadSecondSecret, "127.0.0.1", managedPort, restoreClientBootRouterID, rotatedCAPEM)); err != nil {
		return err
	}
	d2, err := cachedBootStart(ctx, dev2.dir, dev2.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(d2.command, d2.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nedge-02 output:\n%s", retErr, d2.output.String())
		}
	}()

	refusedBefore := cachedBootWait(ctx, d2, fleetHoldLogAttempts, func() bool {
		return strings.Contains(d2.output.String(), "doctor-hub-certificate-unverified")
	})
	if !refusedBefore {
		return errors.New("edge-02 did not refuse the hub's issued leaf before certificate rotation")
	}

	// Rotate hub certificate by reload to fleet-hub.
	rotatedHub := hubConfigServingHost(sshPort, pluginPort, "127.0.0.1", managedPort, pkiBlock, "fleet-hub", clients)
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, caTrustHubName+".conf", []byte(rotatedHub)); err != nil {
		return fmt.Errorf("reload hub setting certificate fleet-hub: %w", err)
	}

	var finalAnswer string
	rotatedOK := cachedBootWait(ctx, d2, fleetHoldDoctorAttempts, func() bool {
		var reqErr error
		finalAnswer, reqErr = storageLiveRequest(ctx, hubOp, "show fleet devices | raw")
		if reqErr != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(finalAnswer, &rows) != nil {
			return false
		}
		var e1OK, e2OK bool
		for _, r := range rows {
			if r.Name == caTrustClientName && r.Connected != nil && *r.Connected && r.ConnectedSince == edge1Since {
				e1OK = true
			}
			if r.Name == fleetReloadSecondClient && r.Connected != nil && *r.Connected && r.ConfigState == "in-sync" {
				e2OK = true
			}
		}
		return e1OK && e2OK
	})
	if !rotatedOK {
		return fmt.Errorf("after rotation edge-02 did not connect or edge-01 changed connected-since (want %s): %s", edge1Since, finalAnswer)
	}
	return storageLiveOK("reload rotated managed certificate for new handshakes while keeping existing session")
}

// managedFleetCertificateRefusedDoctorDriver validates AC-48 and A-15 across
// real hub and client daemons: rotating to an untrusted certificate + leaving
// and rejoining on edge-01 surfaces doctor-hub-certificate-unverified on
// edge-01 and doctor-managed-certificate-refused on the hub, and removing the
// certificate name clears both rows once edge-01 reconnects.
func managedFleetCertificateRefusedDoctorDriver(ctx context.Context, args []string) (retErr error) {
	if len(args) != 3 {
		return errors.New("usage: managed/fleet-certificate-refused-doctor <hub-ssh-port> <hub-plugin-port> <hub-managed-port>")
	}
	sshPort, pluginPort, managedPort := args[0], args[1], args[2]
	managedNum, err := strconv.Atoi(managedPort)
	if err != nil {
		return fmt.Errorf("hub-managed-port %q: %w", managedPort, err)
	}
	ports, _, err := runner.ReservePorts(managedNum+1, 1)
	if err != nil {
		return fmt.Errorf("reserve device SSH port: %w", err)
	}
	defer ports.Release()
	deviceSSHPort := strconv.Itoa(ports.Start)

	pkiBlock, _, err := generateRotatedPKI("fleet-hub")
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "fleet-cert-doctor-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := restoreClientDaemon(ctx, base, "hub", false)
	if err != nil {
		return err
	}
	hub.env = childEnvironment(hub.env, map[string]string{"ze.log.hub": logLevelInfo})
	clients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret + "\"; }\n"
	initialHub := hubConfigServingHost(sshPort, pluginPort, "127.0.0.1", managedPort, pkiBlock, "", clients)
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf", initialHub); err != nil {
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
	hubOp, err := storageLiveClient(ctx, []string{sshPort})
	if err != nil {
		return err
	}
	hubImporter := fleetHoldOperatorEnv(hub.env, sshPort)
	hubRootPEM, err := exportHubLocalCAPEM(ctx, hubOp)
	if err != nil {
		return err
	}

	devSSH := restoreClientSSHListener(deviceSSHPort)
	servedCfg := verifyingClientConfig(devSSH, caTrustClientName, caTrustClientSecret, "127.0.0.1", managedPort, restoreClientServedRouterID, hubRootPEM)
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, "client-"+caTrustClientName+".conf", []byte(servedCfg)); err != nil {
		return err
	}
	device, err := verifyingClientDaemon(ctx, base, "device", caTrustClientName)
	if err != nil {
		return err
	}
	bootCfg := verifyingClientConfig(devSSH, caTrustClientName, caTrustClientSecret, "127.0.0.1", managedPort, restoreClientBootRouterID, hubRootPEM)
	if err := caTrustImportConfig(ctx, device, caTrustClientName+".conf", bootCfg); err != nil {
		return err
	}
	daemon, err := cachedBootStart(ctx, device.dir, device.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\ndevice output:\n%s", retErr, daemon.output.String())
		}
	}()
	deviceOp := fleetHoldOperatorEnv(device.env, deviceSSHPort)

	// 1. edge-01 connects and syncs against the hub's issued leaf.
	connected := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		ans, reqErr := storageLiveRequest(ctx, hubOp, "show fleet devices | raw")
		if reqErr != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(ans, &rows) != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		return rows[0].Name == caTrustClientName && rows[0].Connected != nil && *rows[0].Connected && rows[0].ConfigState == "in-sync"
	})
	if !connected {
		return errors.New("edge-01 never connected to hub before certificate rotation")
	}

	// 2. Rotate hub to certificate fleet-hub (issued by a CA edge-01 does not hold).
	rotatedHub := hubConfigServingHost(sshPort, pluginPort, "127.0.0.1", managedPort, pkiBlock, "fleet-hub", clients)
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, caTrustHubName+".conf", []byte(rotatedHub)); err != nil {
		return fmt.Errorf("reload hub setting fleet-hub: %w", err)
	}

	// 3. Make edge-01 reconnect without restarting its daemon (request fleet disable, then enable).
	if _, err := storageLiveRequest(ctx, deviceOp, fleetLeaveDisable); err != nil {
		return fmt.Errorf("request fleet disable on edge-01: %w", err)
	}
	if _, err := storageLiveRequest(ctx, deviceOp, fleetLeaveEnable); err != nil {
		return fmt.Errorf("request fleet enable on edge-01: %w", err)
	}

	// 4. Assert both sides report the certificate refusal in show doctor, and hub lists edge-01 offline.
	var devDoctor, hubDoctor string
	bothReported := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		devDoctor, _ = storageLiveRequest(ctx, deviceOp, "show doctor") //nolint:errcheck // doctor warnings may exit non-zero
		hubDoctor, _ = storageLiveRequest(ctx, hubOp, "show doctor")    //nolint:errcheck // doctor warnings may exit non-zero
		devOK := strings.Contains(devDoctor, "doctor-hub-certificate-unverified") && strings.Contains(devDoctor, "did not issue it")
		hubOK := strings.Contains(hubDoctor, "doctor-managed-certificate-refused") && strings.Contains(hubDoctor, "127.0.0.1") && strings.Contains(hubDoctor, caTrustClientName)
		if !devOK {
			return false
		}
		if !hubOK {
			return false
		}
		ans, reqErr := storageLiveRequest(ctx, hubOp, "show fleet devices | raw")
		if reqErr != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(ans, &rows) != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		return rows[0].Connected != nil && !*rows[0].Connected
	})
	if !bothReported {
		return fmt.Errorf("certificate refusal not reported on both sides.\ndevice doctor:\n%s\nhub doctor:\n%s", devDoctor, hubDoctor)
	}

	// 5. Second reload removes certificate fleet-hub; edge-01 reconnects and clears both doctor rows.
	if err := fleetHoldImportBeside(ctx, hubImporter, hub, caTrustHubName+".conf", []byte(initialHub)); err != nil {
		return fmt.Errorf("reload hub removing fleet-hub: %w", err)
	}
	bothCleared := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		ans, reqErr := storageLiveRequest(ctx, hubOp, "show fleet devices | raw")
		if reqErr != nil {
			return false
		}
		var rows []fleetRegistryRow
		if fleetDivergenceDecode(ans, &rows) != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		if rows[0].Connected == nil {
			return false
		}
		if !*rows[0].Connected {
			return false
		}
		devDoctor, _ = storageLiveRequest(ctx, deviceOp, "show doctor") //nolint:errcheck // check absence of cert warning
		hubDoctor, _ = storageLiveRequest(ctx, hubOp, "show doctor")    //nolint:errcheck // check absence of cert warning
		return !strings.Contains(devDoctor, "doctor-hub-certificate-unverified") &&
			!strings.Contains(hubDoctor, "doctor-managed-certificate-refused")
	})
	if !bothCleared {
		return fmt.Errorf("after removing certificate name, rows did not clear.\ndevice doctor:\n%s\nhub doctor:\n%s", devDoctor, hubDoctor)
	}
	return storageLiveOK("certificate refusal reported on both client and hub doctor and cleared after repair")
}
