//go:build linux

package iface_test

// The netlink backend registers itself here for the whole test binary: the
// kernel tests of package iface load it by name, and run in the namespace
// userns.Enter gives them, so they build under linux alone.
import _ "github.com/ze-software/ze/internal/plugins/iface/netlink"
