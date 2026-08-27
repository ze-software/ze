//go:build linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports

package qemu

import "github.com/ze-software/ze/internal/core/textbuf"

// VRRPParityValues is the protocol and timing record compared with the Python
// producer while both implementations remain in the tree.
type VRRPParityValues struct {
	VIP, ZeAddress, KAAddress, OBAddress, VirtualMAC, Multicast string
	VRID, ZePriority, KAPriority, AdvertMS, AdvertCS, AdvertTTL int
	QS2PromoteMin, QS2PromoteMax, QS2PreemptMin, QS2PreemptMax  float64
	QS3PromoteMax, QS3NoSkewPath                                float64
}

// VRRPParitySnapshot answers the values that govern the wire verdicts.
func VRRPParitySnapshot() VRRPParityValues {
	return VRRPParityValues{
		VIP: vrrpVIP, ZeAddress: vrrpZeAddress, KAAddress: vrrpKAAddress,
		OBAddress: vrrpOBAddress, VirtualMAC: vrrpVirtualMAC, Multicast: vrrpMulticastV4,
		VRID: vrrpVRID, ZePriority: vrrpZePriority, KAPriority: vrrpKAPriority,
		AdvertMS: vrrpAdvertMS, AdvertCS: vrrpAdvertCS, AdvertTTL: vrrpAdvertTTL,
		QS2PromoteMin: vrrpQS2PromoteMin, QS2PromoteMax: vrrpQS2PromoteMax,
		QS2PreemptMin: vrrpQS2PreemptMin, QS2PreemptMax: vrrpQS2PreemptMax,
		QS3PromoteMax: vrrpQS3PromoteMax, QS3NoSkewPath: vrrpQS3NoSkewPath,
	}
}

// VRRPParityConfigs answers the exact generated files for fixed peer paths.
func VRRPParityConfigs(zeVeth, kaVeth, notify, marker string) ([]byte, []byte) {
	names := vrrpNames{zeVeth: zeVeth, kaVeth: kaVeth}
	return vrrpZeConfig(names), vrrpKeepalivedConfig(names, notify, marker, vrrpKAPriority)
}

// PPPoEParityConfigs answers the exact generated files for one scratch path.
func PPPoEParityConfigs(work, zeVeth, acVeth string) ([]byte, []byte, []byte) {
	var tb textbuf.Buffer
	chap := tb.Str(pppoeUsername).Str("\t*\t").Str(pppoePassword).Str("\t*\n").Bytes()
	return pppoeAccelConfig(work, acVeth), pppoeZeConfig(zeVeth), chap
}

// NetnsParitySelections clones the curated name populations.
func NetnsParitySelections() map[string][]string {
	out := make(map[string][]string, len(netnsSelections))
	for suite, names := range netnsSelections {
		out[suite] = append([]string(nil), names...)
	}
	return out
}
