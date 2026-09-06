// Design: docs/architecture/l2tp/cos-vendor-radius.md -- vendor VSA CoS/rate extraction
// Related: extract.go -- extractAuthMetadata calls extractVSACoSProfile
// Related: coa.go -- extractCoSProfile/extractRate call VSA functions

package l2tpauthradius

import (
	"math"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// extractVSACoSProfile scans Vendor-Specific attributes for a CoS
// profile name. Returns the first match across Cisco, Juniper, Nokia,
// and Huawei VSAs. Unknown vendors are silently ignored.
func extractVSACoSProfile(pkt *radius.Packet) string {
	for _, raw := range pkt.FindAllAttr(radius.AttrVendorSpecific) {
		vendorID, vendorType, value, err := radius.DecodeVSA(raw)
		if err != nil || len(value) == 0 {
			continue
		}
		if name := matchVendorCoS(vendorID, vendorType, value); name != "" {
			return name
		}
	}
	return ""
}

// extractVSARate scans Vendor-Specific attributes for a MikroTik
// rate value. Returns download rate in bits per second, or 0 if not found.
func extractVSARates(pkt *radius.Packet) (download, upload uint64) {
	for _, raw := range pkt.FindAllAttr(radius.AttrVendorSpecific) {
		vendorID, vendorType, value, err := radius.DecodeVSA(raw)
		if err != nil || len(value) == 0 {
			continue
		}
		if vendorID == radius.VendorMikrotik && vendorType == radius.MikrotikRateLimit {
			dl, ul := parseMikrotikRate(value)
			if dl > 0 {
				return dl, ul
			}
		}
	}
	return 0, 0
}

func matchVendorCoS(vendorID uint32, vendorType uint8, value []byte) string {
	switch vendorID {
	case radius.VendorCisco:
		if vendorType == radius.CiscoAVPair {
			name, ok := parseCiscoAVPairCoS(value)
			if ok {
				return name
			}
		}
	case radius.VendorJuniper:
		if vendorType == radius.ERXIngressPolicyName || vendorType == radius.ERXEgressPolicyName {
			return string(value)
		}
	case radius.VendorNokia:
		if vendorType == radius.AlcSubscriberQoSOverride {
			return string(value)
		}
	case radius.VendorHuawei:
		if vendorType == radius.HWSubscriberQoSProfile {
			return string(value)
		}
	}
	return ""
}

const (
	ciscoQoSPolicyInPrefix  = "subscriber:sub-qos-policy-in="
	ciscoQoSPolicyOutPrefix = "subscriber:sub-qos-policy-out="
)

// parseCiscoAVPairCoS extracts a QoS profile name from a Cisco-AVPair
// value. Returns the profile name and true if the AVPair contains a
// recognized QoS policy key. Splits on the first "=" to handle values
// containing "=" characters.
func parseCiscoAVPairCoS(value []byte) (string, bool) {
	s := string(value)
	if strings.HasPrefix(s, ciscoQoSPolicyInPrefix) {
		name := s[len(ciscoQoSPolicyInPrefix):]
		if name != "" {
			return name, true
		}
		return "", false
	}
	if strings.HasPrefix(s, ciscoQoSPolicyOutPrefix) {
		name := s[len(ciscoQoSPolicyOutPrefix):]
		if name != "" {
			return name, true
		}
		return "", false
	}
	if strings.Contains(s, "=") {
		key, _, _ := strings.Cut(s, "=")
		logger().Debug("cisco-avpair: unrecognized key", "key", key)
	}
	return "", false
}

// parseMikrotikRate parses a MikroTik Mikrotik-Rate-Limit value.
// Format: "rx-rate[/tx-rate] [burst fields...]"
// Rate suffixes: k (1000), M (1000000), G (1000000000); plain = bps.
// Returns download and upload rates in bits per second.
func parseMikrotikRate(value []byte) (download, upload uint64) {
	s := strings.TrimSpace(string(value))
	if s == "" {
		return 0, 0
	}
	ratePart, _, _ := strings.Cut(s, " ")

	parts := strings.SplitN(ratePart, "/", 2)
	download = parseMikrotikRateValue(parts[0])
	if len(parts) == 2 {
		upload = parseMikrotikRateValue(parts[1])
	} else {
		upload = download
	}
	return download, upload
}

func parseMikrotikRateValue(s string) uint64 {
	if s == "" {
		return 0
	}
	var mult uint64 = 1
	switch {
	case strings.HasSuffix(s, "G"):
		mult = 1_000_000_000
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "M"):
		mult = 1_000_000
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "k"):
		mult = 1_000
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || (mult > 1 && n > math.MaxUint64/mult) {
		return 0
	}
	return n * mult
}

// mikrotikRateToFilterID converts a MikroTik rate pair in bps to a string that
// traffic.ParseFilterIDRate can parse.
//
// Both directions are written. The MikroTik Rate-Limit attribute carries
// rx/tx, the shaper now enforces both, and a rewrite that kept only the
// download rate lost the subscriber's upload limit before the shaper saw it.
func mikrotikRateToFilterID(downloadBps, uploadBps uint64) string {
	if downloadBps == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Uint(downloadBps).Str("bit")
	if uploadBps > 0 && uploadBps != downloadBps {
		tb.Byte('/').Uint(uploadBps).Str("bit")
	}
	return tb.String()
}
