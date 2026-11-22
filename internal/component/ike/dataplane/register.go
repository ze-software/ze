package dataplane

func init() {
	if err := Register("xfrm", newXFRMBackend); err != nil {
		panic("BUG: dataplane: xfrm registration failed: " + err.Error())
	}
	// The vpp backend registers from register_vpp.go (//go:build ze_vpp); a
	// vpp-less build rejects `dataplane vpp` as an unknown backend.
	// Test infrastructure: side-effect-free backend for unprivileged
	// control-plane tests (selected via ze.test.ike.dataplane=noop; see
	// noop.go for why EPERM on the real xfrm backend stays fatal instead).
	if err := Register("noop", newNoopBackend); err != nil {
		panic("BUG: dataplane: noop registration failed: " + err.Error())
	}
}
