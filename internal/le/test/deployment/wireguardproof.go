// Design: docs/architecture/testing/qemu-integration.md -- real kernel WireGuard ping.
// Related: vpnproof.go -- the VPN evidence action; the separate WireGuard runtime
// spec owns configuration-commit coverage, which this kernel proof does not claim.
package testdeployment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	interopipsec "github.com/ze-software/ze/internal/le/interoplab/ipsec"
)

// The WireGuard proof lab: its two nodes, their tunnel addresses, and the image
// each node runs. capabilityNetAdmin is the Docker capability a lab container
// needs to configure its own links.
const (
	capabilityNetAdmin   = "NET_ADMIN"
	wireguardImage       = "wireguard"
	wireguardLink        = "wg-proof"
	wireguardLeft        = "left"
	wireguardRight       = "right"
	wireguardLeftTunnel  = "10.88.0.1"
	wireguardRightTunnel = "10.88.0.2"
)

func runWireGuardProof(ctx context.Context, root string) interoplab.SuiteReport {
	suffix := strconv.Itoa(os.Getpid())
	peers := make([]interoplab.PeerConfig, 0, 2)
	for index, name := range []string{wireguardLeft, wireguardRight} {
		peers = append(peers, interoplab.PeerConfig{
			Name: name, Container: "ze-vpn-wireguard-" + name + "-" + suffix,
			Image: wireguardImage, Host: uint8(index + 2),
			Capabilities: []string{capabilityNetAdmin},
			// This is the Docker daemon's root, not the client's filesystem.
			// In the QEMU path it proves absence in the guest, not in a container.
			Mounts: []interoplab.Mount{{Source: "/", Target: "/ze-host", ReadOnly: true}},
		})
	}
	suite := interoplab.Suite{
		Docker: interoplab.NewDocker(), Checkout: root,
		Preflight: interoplab.StageBinaries(root, false, interopipsec.LabBinaries()...),
		StagedZe:  interoplab.StagedZePath(root, interopipsec.LabBinaries()),
		Images: []interoplab.ImageBuild{{
			Name: wireguardImage, Tag: "ze-vpn-wireguard",
			Dockerfile: filepath.Join(root, "internal", "le", "test", "deployment", "testdata", "Dockerfile.wireguard"),
			Context:    root, Required: true,
		}},
		Scenarios: []interoplab.ScenarioPlan{{
			Source: interoplab.ScenarioSource{Name: "wireguard-kernel-ping", Checker: checkWireGuardProof},
			Network: interoplab.NetworkSpec{
				Name:       "ze-vpn-wireguard-" + suffix,
				Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.251.0/24")}},
			},
			Peers: peers,
		}},
	}
	return suite.Run(ctx)
}

func checkWireGuardProof(ctx context.Context, check *interoplab.CheckContext) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	// /proc is a mountpoint even when /sbin does not exist on the appliance.
	// A dangling modprobe symlink is also forbidden, not mistaken for absence.
	if _, err := check.Lab.Exec(ctx, wireguardLeft, []string{"chroot", "/ze-host", "/bin/sh", "-ec",
		"test -d /proc; test ! -e /sbin/modprobe; test ! -L /sbin/modprobe; " +
			"test \"$(cat /proc/sys/kernel/modules_disabled)\" = 1; test -z \"$(cat /proc/modules)\""}, nil); err != nil {
		return fmt.Errorf("guest must have no modprobe or loaded modules, and module loading must be disabled: %w", err)
	}
	if !check.Network.IPv4.IsValid() {
		return errors.New("WireGuard proof has no IPv4 underlay")
	}
	leftAddress := check.Network.IPv4.Addr().Next().Next()
	rightAddress := leftAddress.Next()
	var keys [2]string
	for index, peer := range []string{wireguardLeft, wireguardRight} {
		key, err := check.Lab.Query(ctx, peer, []string{"sh", "-ec",
			"umask 077; wg genkey > /tmp/wg-key; wg pubkey < /tmp/wg-key"}, nil)
		if err != nil {
			return err
		}
		keys[index] = strings.TrimSpace(key)
		decoded, err := base64.StdEncoding.DecodeString(keys[index])
		if err != nil {
			return fmt.Errorf("%s returned an invalid WireGuard public key: %w", peer, err)
		}
		if len(decoded) != 32 {
			return fmt.Errorf("%s WireGuard public key has %d bytes, want 32", peer, len(decoded))
		}
	}
	for _, endpoint := range []struct{ name, local, remote, key, underlay string }{
		{wireguardLeft, wireguardLeftTunnel, wireguardRightTunnel, keys[1], rightAddress.String()},
		{wireguardRight, wireguardRightTunnel, wireguardLeftTunnel, keys[0], leftAddress.String()},
	} {
		for _, argv := range [][]string{
			{"ip", "link", ipAdd, wireguardLink, "type", "wireguard"},
			{"wg", "set", wireguardLink, "private-key", "/tmp/wg-key", "listen-port", "51820",
				"peer", endpoint.key, "allowed-ips", endpoint.remote + "/32", "endpoint", endpoint.underlay + ":51820"},
			{"ip", "address", ipAdd, endpoint.local + "/32", "dev", wireguardLink},
			{"ip", "link", "set", wireguardLink, "up"},
			{"ip", "route", ipAdd, endpoint.remote + "/32", "dev", wireguardLink},
		} {
			if _, err := check.Lab.Exec(ctx, endpoint.name, argv, nil); err != nil {
				return fmt.Errorf("configure %s WireGuard endpoint: %w", endpoint.name, err)
			}
		}
	}
	if _, err := check.Lab.Exec(ctx, wireguardLeft, []string{"ping", "-n", "-I", wireguardLink, "-c", "3", "-W", "5", wireguardRightTunnel}, nil); err != nil {
		return fmt.Errorf("WireGuard tunnel ping: %w", err)
	}
	for index, peer := range []string{wireguardLeft, wireguardRight} {
		handshake, err := check.Lab.Query(ctx, peer, []string{"wg", "show", wireguardLink, "latest-handshakes"}, nil)
		if err != nil {
			return err
		}
		transfer, err := check.Lab.Query(ctx, peer, []string{"wg", "show", wireguardLink, "transfer"}, nil)
		if err != nil {
			return err
		}
		if err := requireWireGuardReceipt(keys[1-index], handshake, transfer); err != nil {
			return fmt.Errorf("%s: %w", peer, err)
		}
	}
	return nil
}

// requireWireGuardReceipt rejects a successful local send with no received data.
// Each namespace starts empty, so any handshake and received byte count are new.
func requireWireGuardReceipt(key, handshake, transfer string) error {
	handshakeFields := strings.Fields(handshake)
	if len(handshakeFields) != 2 {
		return errors.New("WireGuard did not report exactly one peer handshake")
	}
	if handshakeFields[0] != key {
		return errors.New("WireGuard handshake belongs to a different peer")
	}
	when, err := strconv.ParseUint(handshakeFields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("read WireGuard handshake timestamp: %w", err)
	}
	if when == 0 {
		return errors.New("WireGuard peer completed no handshake")
	}
	transferFields := strings.Fields(transfer)
	if len(transferFields) != 3 {
		return errors.New("WireGuard did not report exactly one peer transfer")
	}
	if transferFields[0] != key {
		return errors.New("WireGuard transfer belongs to a different peer")
	}
	received, err := strconv.ParseUint(transferFields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("read WireGuard received byte count: %w", err)
	}
	if received == 0 {
		return errors.New("WireGuard peer received no tunnel bytes")
	}
	return nil
}
