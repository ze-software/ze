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

// ZeCLICommand builds the argv that asks a Ze daemon's CLI one question from
// inside its container: it seeds a client store once (`ze init`), then runs
// `ze cli -c`. The daemon's config MUST carry the `system authentication`
// user and the `environment ssh` server on 127.0.0.1 port 2222 this argv logs
// into, as every topology scenario's Ze config does.
func ZeCLICommand(command string) []string {
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(ZeCLIUser).Byte(' ').
		Str(zeCLIPassword).Byte(' ').Str(ZeCLIPort).Str(" | ZE_CONFIG_DIR=").
		Str(ZeCLIStore).Str(" ze init").String()
	run := tb.Reset().Str("ZE_CONFIG_DIR=").Str(ZeCLIStore).Str(" ZE_SSH_PASSWORD=").
		Str(zeCLIPassword).Str(" ze cli -c ").Str(ShellQuoted(command)).String()
	shell := tb.Reset().Str("[ -d ").Str(ZeCLIStore).Str(" ] || ").Str(seed).
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
