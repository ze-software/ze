package bgp

import (
	"strings"
	"testing"
)

// TestStayRTRReadyCommandRequiresDataAndListener checks that a StayRTR cache
// counts as ready only once its export holds a prefix AND its RTR port accepts.
// Method: read the probe's shell script. StayRTR answers /rpki.json with status
// 200 before it loads its VRP file and binds the RTR port, so a probe of the
// export alone let Ze start against a cache that refused its first query, and
// Ze then waited the 600 s default Retry Interval (RFC 8210 Section 6).
func TestStayRTRReadyCommandRequiresDataAndListener(t *testing.T) {
	command := stayRTRReadyCommand()
	if len(command) != 3 || command[0] != "sh" || command[1] != "-c" {
		t.Fatalf("probe is %q, want sh -c <script>", command)
	}
	script := command[2]
	for _, want := range []string{
		"wget -q -O - http://127.0.0.1:9847/rpki.json",
		`grep -q '"prefix"'`,
		"nc -z 127.0.0.1 8282",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("probe script %q lacks %q", script, want)
		}
	}
	if !strings.Contains(script, "&&") {
		t.Errorf("probe script %q does not require both checks", script)
	}
}
