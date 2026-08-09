//go:build linux

// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- kernel XFRM drop stats (Linux).
// RFC: rfc/short/rfc4552.md (§3/§4 silent discard of unprotected/failed packets).
//
// /proc/net/xfrm_stat exposes node-global XFRM SNMP-style counters. The relevant
// inbound-drop lines map to RFC 4552 §3/§4 silent-discard reasons: a packet that
// arrives without the required policy (unprotected) and a packet that fails the
// integrity/decryption check. These are the drops the OSPFv3 IPsec inbound policy
// performs below the socket, surfaced as a metric (Ze never sees the dropped bytes).

package ospf

import (
	"bufio"
	"os"
	"strconv"
)

// xfrmStatPath is the procfs XFRM counters file; overridable in tests.
var xfrmStatPath = "/proc/net/xfrm_stat"

// xfrmDropReasons maps /proc/net/xfrm_stat field names to the metric reason label.
var xfrmDropReasons = map[string]string{
	"XfrmInNoPols":          "no-policy",   // inbound OSPF packet with no matching (required) policy
	"XfrmInNoStates":        "no-policy",   // inbound protected packet with no matching SA
	"XfrmInStateProtoError": "auth-failed", // transform (ESP/AH) processing error
	"XfrmInIntegFailures":   "auth-failed", // integrity check failed
	"XfrmInStateInvalid":    "auth-failed",
}

func readXfrmDropsPlatform() (map[string]uint64, error) {
	f, err := os.Open(xfrmStatPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	out := make(map[string]uint64)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := splitFields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		reason, ok := xfrmDropReasons[fields[0]]
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		out[reason] += v
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// splitFields splits a whitespace-separated line into its non-empty fields.
func splitFields(line string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(line); i++ {
		if i == len(line) || line[i] == ' ' || line[i] == '\t' {
			if start >= 0 {
				out = append(out, line[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	return out
}
