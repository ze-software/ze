package init

// The production binary links the netlink interface backend through the
// generated composition root (internal/component/plugin/all), so this package
// no longer imports it. Its tests discover interfaces without that root, so
// they link the backend here (spec-plugin-ownership-imports A-2).
import _ "github.com/ze-software/ze/internal/plugins/iface/netlink"
