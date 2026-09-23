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

// Credential-bearing files are owner-only, including pppd's options and Ze's
// local user configuration. xl2tpd also requires a private tunnel secrets file.
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

// writeInputs lays out the peer's files and ze's private configuration directory.
// The daemon creates its database tree beside ze.conf in that directory.
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
		{PeerOptionsFile, l.pppOptions(work, "s3cr3t"), secretsMode},
		{filepath.Join("ze", "ze.conf"), l.daemonConfig(), secretsMode},
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
// evidence used to analyze a failed run. Autodial starts the peer immediately.
// Credentialed attempts do not redial: rejection must end that one session.
func (l *L2TPPPP) peerConfig(work string) string {
	redial := "yes"
	if l.Scenario == "chap-md5" {
		redial = "no"
	}
	var tb textbuf.Buffer
	return tb.Str("[global]\nport = ").Str(l.PeerPort).
		Str("\nauth file = ").Str(filepath.Join(work, PeerSecretsFile)).
		Str("\ndebug tunnel = yes\ndebug state = yes\ndebug packet = yes\ndebug avp = yes\n\n").
		Str("[lac ze]\nlns = ").Str(l.ListenIP).
		Str("\nautodial = yes\nredial = ").Str(redial).Str("\nredial timeout = 1\nmax redials = 5\n").
		Str("require authentication = no\nppp debug = yes\npppoptfile = ").
		Str(filepath.Join(work, PeerOptionsFile)).
		Str("\nlength bit = yes\n").String()
}

// pppOptions answers what xl2tpd hands pppd.
//
// noauth means pppd does not authenticate the LNS; it still answers the LNS's
// CHAP challenge. Refusing the other methods makes the credentialed carrier
// specifically CHAP-MD5. IPv6 is off because this proof has an IPv4-only pool.
func (l *L2TPPPP) pppOptions(work, password string) string {
	var tb textbuf.Buffer
	tb.Str("noauth\nname alice\npassword ").Str(password).Byte('\n')
	if l.Scenario == "chap-md5" {
		tb.Str("refuse-pap\nrefuse-mschap\nrefuse-mschap-v2\n")
	}
	return tb.Str("refuse-eap\nnodefaultroute\n").
		Str("ipcp-accept-local\nipcp-accept-remote\nnoipv6\ndebug\nnodetach\nlogfile ").
		Str(filepath.Join(work, "pppd.log")).Byte('\n').String()
}

// daemonConfig uses the existing local CHAP-MD5 handler only in the credentialed
// carrier. The no-auth carrier keeps its explicit unauthenticated policy.
func (l *L2TPPPP) daemonConfig() string {
	var tb textbuf.Buffer
	tb.Str("l2tp {\n    enabled true;\n")
	if l.Scenario == "chap-md5" {
		tb.Str("    auth-method chap-md5;\n    allow-no-auth false;\n").
			Str("    auth {\n        local {\n            user alice {\n                password s3cr3t;\n            }\n        }\n    }\n")
	} else {
		tb.Str("    auth-method none;\n    allow-no-auth true;\n")
	}
	return tb.Str("    hello-interval 5;\n    max-tunnels 4;\n    max-sessions 4;\n").
		Str("    pool {\n        ipv4 {\n            gateway ").Str(L2TPPPPLocalAddr).
		Str(";\n            start ").Str(L2TPPPPPeerAddr).
		Str(";\n            end ").Str(L2TPPPPPoolEnd).
		Str(";\n            dns-primary 8.8.8.8;\n            dns-secondary 8.8.4.4;\n").
		Str("        }\n    }\n}\nenvironment {\n    l2tp {\n        server main {\n            ip ").
		Str(l.ListenIP).
		Str(";\n            port ").Str(l.ListenPort).
		Str(";\n        }\n    }\n}\n").String()
}
