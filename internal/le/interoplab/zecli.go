// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: ipsec/ipsec.go -- the IPsec lab's ze.conf serves this account too.
package interoplab

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The CLI account a lab's Ze daemon serves its checker on: SSH on the
// container's loopback, as the fixture-only user the scenario's ze.conf
// declares with a published bcrypt hash of zeCLIPassword. ZeCLIStore is the
// client store ZeCLICommand seeds, which a lab running Ze outside a container
// rewrites to a directory of its own.
const (
	ZeCLIStore    = "/tmp/ze-cli-store"
	ZeCLIUser     = "interop"
	zeCLIPassword = "testpass"
	ZeCLIPort     = "2222"
)

// ZeCLILoopback answers the address the fixture account's SSH server answers
// on, the container's IPv4 loopback.
func ZeCLILoopback() netip.Addr {
	return netip.AddrFrom4([4]byte{127, 0, 0, 1})
}

// ZeCLILogin is an account a Ze daemon's SSH server serves inside its
// container, and the client store its checker seeds for it. Host is the
// address the checker dials: the container's own, so the session never leaves
// it, but the source address the kernel then picks is Host itself, and a
// daemon whose input firewall admits SSH only from named sources (the home
// CPE's converted ruleset admits its LAN prefixes, not 127.0.0.0/8) answers
// only on an address inside one of them. Store is the account's own
// directory: `ze init` seeds a store once, so two accounts sharing one would
// log the second in with the first's name.
type ZeCLILogin struct {
	User     string
	Password string
	Host     netip.Addr
	Port     string
	Store    string
}

// ZeCLICommand builds the argv that asks a Ze daemon's CLI one question from
// inside its container, as the fixture-only account above. The daemon's
// config MUST carry that `system authentication` user and the
// `environment ssh` server on 127.0.0.1 port 2222, as every topology
// scenario's hand-written Ze config does.
func ZeCLICommand(command string) []string {
	return ZeCLICommandAs(ZeCLILogin{User: ZeCLIUser, Password: zeCLIPassword, Host: ZeCLILoopback(), Port: ZeCLIPort, Store: ZeCLIStore}, command)
}

// ZeCLICommandAs builds the argv that asks a Ze daemon's CLI one question
// from inside its container as login: it seeds login's client store once
// (`ze init`), then runs `ze cli -c`, with host key verification off when
// login's host is not a loopback address. The daemon's config MUST carry login's
// `system authentication` user and an `environment ssh` server that answers
// on login's host and port. A configuration that is not the lab's own,
// such as the home CPE's converted one, names its account here.
func ZeCLICommandAs(login ZeCLILogin, command string) []string {
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n%s\\n%s\\n' ").Str(ShellQuoted(login.User)).Byte(' ').
		Str(ShellQuoted(login.Password)).Byte(' ').Str(ShellQuoted(login.Host.String())).Byte(' ').
		Str(ShellQuoted(login.Port)).Str(" | ZE_CONFIG_DIR=").
		Str(login.Store).Str(" ze init").String()
	tb.Reset().Str("ZE_CONFIG_DIR=").Str(login.Store)
	// The SSH client trusts an unknown host key only on a loopback host
	// (sshclient.hostKeyCallback), and the lab seeds no host key. A login on
	// a non-loopback address still dials the container's own address, so the
	// session never leaves it and skipping the key check trusts nothing a
	// loopback session would not.
	if !login.Host.IsLoopback() {
		tb.Str(" ZE_SSH_INSECURE=true")
	}
	run := tb.Str(" ZE_SSH_PASSWORD=").Str(ShellQuoted(login.Password)).
		Str(" ze cli -c ").Str(ShellQuoted(command)).String()
	shell := tb.Reset().Str("[ -d ").Str(login.Store).Str(" ] || ").Str(seed).
		Str(" >/dev/null; ").Str(run).String()
	return []string{"sh", "-c", shell}
}

// ShellQuoted quotes value as one POSIX shell word.
func ShellQuoted(value string) string {
	var tb textbuf.Buffer
	tb.Byte('\'')
	for index := range len(value) {
		if value[index] == '\'' {
			tb.Str(`'\''`)
			continue
		}
		tb.Byte(value[index])
	}
	return tb.Byte('\'').String()
}
