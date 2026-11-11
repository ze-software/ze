// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane
// Related: misc_fixture_vpp.go -- the VPP stub, the ze process and the stub log reader this driver uses
//
// vpp/vpp-mpls-ldp-pop drives `ldp { }` under `interface { backend vpp; }`
// with no `fib { }` block against the VPP stub, and passes only when the
// stub log holds MPLS table 0 created before two label routes for one LDP
// local label: the S=1 (eos 1) and S=0 (eos 0) routes of the egress pop for
// the LSR-ID FEC, both in MPLS table 0. Only fib-vpp speaks to the VPP API
// socket, so the routes prove the engine loaded fib-vpp as the writer of the
// vpp data plane and that LDP's entry reached it.

package fixture

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	Register("vpp/vpp-mpls-ldp-pop", vppMPLSLDPPopDriver)
}

// vppMPLSLDPPopConfig is LDP on loopback with the VPP backend and no fib block:
// the engine, not the config, has to choose the label writer.
func vppMPLSLDPPopConfig(socket string) string {
	var tb textbuf.Buffer
	tb.Str("environment {\n}\nvpp { enabled true; external true; api-socket ").Str(socket).Str("; }\n").
		Str("interface { backend vpp; }\n").
		Str("ldp { lsr-id 10.0.0.1; transport-address 10.0.0.1; interfaces lo; }\n")
	return tb.String()
}

// mplsLDPPopEvidence is what the stub log shows for LDP's local label.
type mplsLDPPopEvidence struct {
	tableAt int               // log line of the first mpls_table_add_del table 0 add; -1 if none
	eosAt   map[uint32][2]int // per label: line of the eos 1 add and of the eos 0 add, -1 if absent
}

// readMPLSLDPPopEvidence scans the stub log once.
func readMPLSLDPPopEvidence(log string) (mplsLDPPopEvidence, error) {
	evidence := mplsLDPPopEvidence{tableAt: -1, eosAt: map[uint32][2]int{}}
	line := 0
	_, err := vppLogMatches(log, func(entry map[string]any) bool {
		line++
		fields, _ := entry["fields"].(map[string]any)
		if fields["is_add"] != true {
			return false
		}
		switch entry["msg"] {
		case "mpls_table_add_del":
			if fields["table_id"] == float64(0) && evidence.tableAt < 0 {
				evidence.tableAt = line
			}
		case "mpls_route_add_del":
			recordMPLSLDPPopRoute(&evidence, fields, line)
		}
		return false
	})
	return evidence, err
}

// recordMPLSLDPPopRoute notes the line of one table 0 label route add.
func recordMPLSLDPPopRoute(evidence *mplsLDPPopEvidence, fields map[string]any, line int) {
	if fields["table_id"] != float64(0) {
		return
	}
	label, isNumber := fields["label"].(float64)
	if !isNumber {
		return
	}
	seen, known := evidence.eosAt[uint32(label)]
	if !known {
		seen = [2]int{-1, -1}
	}
	switch fields["eos"] {
	case float64(1):
		seen[0] = line
	case float64(0):
		seen[1] = line
	}
	evidence.eosAt[uint32(label)] = seen
}

// poppedLabel answers the label holding both stack positions after table 0
// was created, or false.
func (e mplsLDPPopEvidence) poppedLabel() (uint32, bool) {
	if e.tableAt < 0 {
		return 0, false
	}
	for label, at := range e.eosAt {
		if at[0] > e.tableAt && at[1] > e.tableAt {
			return label, true
		}
	}
	return 0, false
}

func vppMPLSLDPPopDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("vpp-mpls-ldp-pop fixture takes no arguments")
	}
	dir, socket, log, err := vppWorkPaths("vpp-ldp-pop-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	stub, err := startVPPStub(ctx, socket, log, 30)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	ze, err := startVPPZe(ctx, vppMPLSLDPPopConfig(socket), map[string]string{
		envConfigDir: dir, envLogVPP: logLevelInfo, "ze.log.fib.vpp": logLevelDebug, envLogBGP: logLevelWarn,
	})
	if err != nil {
		return err
	}
	var evidence mplsLDPPopEvidence
	var scanErr error
	Poll(ctx, 150, 100*time.Millisecond, func() bool {
		evidence, scanErr = readMPLSLDPPopEvidence(log)
		_, found := evidence.poppedLabel()
		return found || scanErr != nil
	})
	stopFixtureProcess(ze, 3*time.Second)
	if scanErr != nil {
		return scanErr
	}
	label, found := evidence.poppedLabel()
	if !found {
		var dump textbuf.Buffer
		dump.Str("--- ze output ---\n").Str(ze.output.String()).Str("--- end ---\n")
		if data, readErr := os.ReadFile(log); readErr == nil { //nolint:gosec // the path is the fixture's own scratch file
			dump.Str("--- stub log ---\n").Str(string(data)).Str("--- end ---\n")
		}
		_ = dump.StdErr() //nolint:errcheck // evidence for a failure already being returned
		return errors.New("stub did not observe MPLS table 0 created, then an eos 1 and an eos 0 add in table 0 for one LDP label")
	}
	var ok textbuf.Buffer
	ok.Str("OK: fib-vpp created MPLS table 0 and programmed LDP label ").Uint32(label).Str(" as eos and neos routes in table 0\n")
	if err := ok.StdErr(); err != nil {
		return err
	}
	return nil
}
