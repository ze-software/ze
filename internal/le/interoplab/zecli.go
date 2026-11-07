// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: ipsec/ipsec.go -- the IPsec lab's ze.conf serves this account too.
package interoplab

import "github.com/ze-software/ze/internal/core/textbuf"

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

// ZeCLILogin is an account a Ze daemon's SSH server serves on the
// container's loopback, and the client store its checker seeds for it. Store
// is the account's own directory: `ze init` seeds a store once, so two
// accounts sharing one would log the second in with the first's name.
type ZeCLILogin struct {
	User     string
	Password string
	Port     string
	Store    string
}

// ZeCLICommand builds the argv that asks a Ze daemon's CLI one question from
// inside its container, as the fixture-only account above. The daemon's
// config MUST carry that `system authentication` user and the
// `environment ssh` server on 127.0.0.1 port 2222, as every topology
// scenario's hand-written Ze config does.
func ZeCLICommand(command string) []string {
	return ZeCLICommandAs(ZeCLILogin{User: ZeCLIUser, Password: zeCLIPassword, Port: ZeCLIPort, Store: ZeCLIStore}, command)
}

// ZeCLICommandAs builds the argv that asks a Ze daemon's CLI one question
// from inside its container as login: it seeds login's client store once
// (`ze init`), then runs `ze cli -c`. The daemon's config MUST carry login's
// `system authentication` user and an `environment ssh` server that answers
// on 127.0.0.1 at login's port. A configuration that is not the lab's own,
// such as the home CPE's converted one, names its account here.
func ZeCLICommandAs(login ZeCLILogin, command string) []string {
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(ShellQuoted(login.User)).Byte(' ').
		Str(ShellQuoted(login.Password)).Byte(' ').Str(ShellQuoted(login.Port)).Str(" | ZE_CONFIG_DIR=").
		Str(login.Store).Str(" ze init").String()
	run := tb.Reset().Str("ZE_CONFIG_DIR=").Str(login.Store).Str(" ZE_SSH_PASSWORD=").
		Str(ShellQuoted(login.Password)).Str(" ze cli -c ").Str(ShellQuoted(command)).String()
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
