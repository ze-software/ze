// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: afterline.go -- commandWordAt, the walk past the launchers heavytool declares
// Related: session_identity_test.go -- runSubagentBash and identityLineOfAgent, the route these rows take

package hookruntime

import (
	"strings"
	"testing"
)

// TestIdentityGuardReadsTheLaunchersHeavytoolDeclares asks whether the
// identity guard reads past every launcher the one declaration holds
// (heavytool.LauncherNamed), with the arguments it declares, so an
// environment tool behind doas, taskset, ionice or a tracer is found, and
// whether doas and su are refused as sudo is. Its method: each line goes
// through the hook's Run after the agent's own identity line, on a line that
// parses and on one the parser refuses (the unclosed quote), and the
// survivors are the same launchers in front of `./le`.
//
// VALIDATES: `doas ./le x`, `su u -c './le x'`, `taskset ff env -i ./le x`,
// `ionice -c 3 env -i ./le x` and `strace --argv0 x env -i ./le x` are
// refused as taking another identity; `taskset ff ./le x`, `timeout inf ./le
// x` and `/usr/bin/nohup ./le x` pass.
// PREVENTS: a second table of wrappers in the identity guard that lacks a
// launcher the heavy reading holds (doas, ionice, taskset), so the tool
// behind it is never judged.
func TestIdentityGuardReadsTheLaunchersHeavytoolDeclares(t *testing.T) {
	for _, rest := range []string{
		`doas ./le x`,
		`/usr/bin/doas -u u ./le x`,
		`doas -n ./le spec current`,
		`su u -c './le x'`,
		`/bin/su - u`,
		`taskset ff env -i ./le x`,
		`taskset 0x3 env -i ./le x`,
		`taskset -c 0-3 env -i ./le x`,
		`ionice -c 3 env -i ./le x`,
		`ionice -c3 -n7 sudo ./le x`,
		`strace --argv0 x env -i ./le x`,
		`strace -e trace=open env -i ./le x`,
		`timeout inf env -i ./le x`,
		`timeout 1e3 env -i ./le x`,
		`timeout 1.5 env -i ./le x`,
		`timeout 10s env -i ./le x`,
		`timeout 5m env -i ./le x`,
		`/usr/bin/nohup env -i ./le x`,
		`/usr/bin/timeout 5 /usr/bin/env -i ./le x`,
		`xargs -0n 1 env -i ./le x`,
		// The text route, which judges a remainder the parser refuses.
		`doas ./le x 'oops`,
		`taskset ff env -i ./le x 'oops`,
		`ionice -c 3 env -i ./le x 'oops`,
		`timeout inf env -i ./le x 'oops`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", rest, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", rest, message)
		}
	}
	for _, rest := range []string{
		`taskset ff ./le x`,
		`taskset -c 0-3 ./le x`,
		`ionice -c 3 ./le x`,
		`timeout inf ./le x`,
		`timeout 1e3 ./le spec current`,
		`/usr/bin/nohup ./le x`,
		`strace --argv0 x ./le x`,
		`flock -w 5 l.lock ./le x`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", rest, code, output, message)
		}
	}
}
