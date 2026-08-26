// Design: docs/architecture/testing/interop.md -- the peer's own configuration
// Overview: l2tpppp.go -- the run that writes these files and points both daemons at them
//
// l2tppppinputs.go writes the files that the two daemons read. The peer's three
// files use xl2tpd's own format and pppd's own format. ze's file uses ze's own
// format. The four configurations are stated here instead of at process start.
// The proof asserts about what these files request, and a reader checks them
// against a verdict.
//
// Every path in the files is absolute. Each daemon runs in its own network
// namespace and uses the host's filesystem. The run's scratch directory is the
// one thing that both daemons can see.

package deployment

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two permissions the run writes with. The secrets file is narrowed because
// xl2tpd refuses one anybody can read, and it is the only file here that
// carries a credential.
const (
	inputMode   os.FileMode = 0o644
	secretsMode os.FileMode = 0o600
)

// These are the three files that an xl2tpd peer reads. Every proof in this
// package writes the same three files under the same names. They use xl2tpd's
// and pppd's own grammar, not a format chosen by one proof.
const (
	PeerConfigFile  = "xl2tpd.conf"
	PeerSecretsFile = "l2tp-secrets" //nolint:gosec // G101: the NAME of a file, not a credential
	PeerOptionsFile = "ppp-options"
)

// L2TPPPPSecrets is the xl2tpd secrets file. The tunnel does not authenticate,
// so its one entry matches any peer. This proof concerns the PPP path above the
// tunnel. An authentication failure would hide that path.
const L2TPPPPSecrets = "* * s3cr3t\n"

// writeInputs lays out the directory both daemons read from: the peer's three
// files, ze's configuration, and the empty directory ze writes its
// configuration store into.
func (l *L2TPPPP) writeInputs(work string) error {
	if err := os.MkdirAll(filepath.Join(work, "ze"), 0o750); err != nil {
		return err
	}

	files := []struct {
		name string
		body string
		mode os.FileMode
	}{
		{PeerConfigFile, l.peerConfig(work), inputMode},
		{PeerSecretsFile, L2TPPPPSecrets, secretsMode},
		{PeerOptionsFile, l.pppOptions(work), inputMode},
		{"ze.conf", l.daemonConfig(), inputMode},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(work, file.name), []byte(file.body), file.mode); err != nil {
			return err
		}
	}
	return nil
}

// peerConfig answers the xl2tpd configuration.
//
// Every debug switch is on because the peer's account of the tunnel is the
// evidence used to analyze a failed run. Autodial and a short redial make the
// peer dial as soon as it starts rather than wait to be told.
func (l *L2TPPPP) peerConfig(work string) string {
	var tb textbuf.Buffer
	return tb.Str("[global]\nport = ").Str(l.PeerPort).
		Str("\nauth file = ").Str(filepath.Join(work, PeerSecretsFile)).
		Str("\ndebug tunnel = yes\ndebug state = yes\ndebug packet = yes\ndebug avp = yes\n\n").
		Str("[lac ze]\nlns = ").Str(l.ListenIP).
		Str("\nautodial = yes\nredial = yes\nredial timeout = 1\nmax redials = 5\n").
		Str("require authentication = no\nppp debug = yes\npppoptfile = ").
		Str(filepath.Join(work, PeerOptionsFile)).
		Str("\nlength bit = yes\n").String()
}

// pppOptions answers what xl2tpd hands pppd.
//
// pppd refuses EAP and accepts both IPCP addresses from the far end. Thus, the
// PPP layer cannot be the thing that fails. The proof verifies that ze drives
// IPCP to completion, not that pppd can negotiate against a policy. IPv6 is off
// because the pool this proof configures hands out v4 only. nodetach keeps
// pppd's output on the peer's own stream.
func (l *L2TPPPP) pppOptions(work string) string {
	var tb textbuf.Buffer
	return tb.Str("noauth\nname alice\npassword s3cr3t\nrefuse-eap\nnodefaultroute\n").
		Str("ipcp-accept-local\nipcp-accept-remote\nnoipv6\ndebug\nnodetach\nlogfile ").
		Str(filepath.Join(work, "pppd.log")).Byte('\n').String()
}

// daemonConfig answers ze's configuration: an L2TP server on the underlay, an
// address pool with a gateway and a range, and no authentication.
func (l *L2TPPPP) daemonConfig() string {
	var tb textbuf.Buffer
	return tb.Str("l2tp {\n    enabled true;\n    auth-method none;\n    allow-no-auth true;\n").
		Str("    hello-interval 5;\n    max-tunnels 4;\n    max-sessions 4;\n").
		Str("    pool {\n        ipv4 {\n            gateway ").Str(L2TPPPPLocalAddr).
		Str(";\n            start ").Str(L2TPPPPPeerAddr).
		Str(";\n            end ").Str(L2TPPPPPoolEnd).
		Str(";\n            dns-primary 8.8.8.8;\n            dns-secondary 8.8.4.4;\n").
		Str("        }\n    }\n}\nenvironment {\n    l2tp {\n        server main {\n            ip ").
		Str(l.ListenIP).
		Str(";\n            port ").Str(l.ListenPort).
		Str(";\n        }\n    }\n}\n").String()
}
