// Fixture for RulePathIDZero (AC-3).
package fixture

type route struct {
	PathID  uint32
	AddPath bool
}

func keyFromField(r route) bool {
	return r.PathID != 0 // want: path-id-compared-with-zero
}

func keyFromValue(pathID uint32) bool {
	hasPathID := pathID != 0 // want: path-id-compared-with-zero
	return hasPathID
}

func keyReversed(nlriPathId uint32) bool {
	return 0 == nlriPathId // want: path-id-compared-with-zero
}

// keyFromCapability reads the negotiated capability: no finding.
func keyFromCapability(r route) bool {
	return r.AddPath
}

// labelZero compares something that is not a path identifier: no finding.
func labelZero(label uint32) bool {
	return label != 0
}
