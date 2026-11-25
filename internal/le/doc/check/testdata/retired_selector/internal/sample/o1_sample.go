package netlink
func f() {
	tc := 1
	_ = tc
	_ = LoadBackend("tc")
	// defaults to netlink
}
