// Design: docs/features/ai-first.md -- RADIUS server reachability doctor check
// Related: register.go -- doctor check registration (l2tp-auth-radius-servers)
// Related: config.go -- l2tp.auth.radius config shape this check walks

package l2tpauthradius

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// udpReachable is a test seam over udpServerReachable.
var udpReachable = udpServerReachable

// checkRADIUSServers probes every server under l2tp.auth.radius with a RADIUS
// Access-Request and emits doctor-radius-unreachable when none responds.
// This plugin owns the l2tp.auth.radius config block, so it owns the
// readiness check (see ai/rules/doctor-checks.md "Where to Register Checks").
func checkRADIUSServers(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	radiusCfg := radiusAuthContainer(tree)
	if radiusCfg == nil {
		return nil
	}

	timeout := doctorTimeout(radiusCfg, "timeout", 3)
	nasID := doctorValueOrDefault(radiusCfg, "nas-identifier", "ze-doctor")
	var sourceIP net.IP
	if source, ok := radiusCfg.Get("source-address"); ok && source != "" {
		sourceIP = net.ParseIP(source)
	}
	checked := false
	for _, s := range radiusCfg.GetListOrdered("server") {
		address, ok := s.Value.Get("address")
		if !ok || address == "" {
			continue
		}
		checked = true
		secret := []byte(doctorValueOrDefault(s.Value, "shared-key", ""))
		if udpReachable(net.JoinHostPort(address, doctorValueOrDefault(s.Value, "port", "1812")), secret, sourceIP, nasID, timeout) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-radius-unreachable",
		Severity: "warning",
		Message:  "none of the configured RADIUS servers are reachable",
	}}
}

// udpServerReachable sends an authenticated Access-Request and reports whether
// the server returned a verifiable response. Dial success alone is not enough:
// an unbound UDP port or a wrong shared key must count as unreachable.
func udpServerReachable(addr string, secret []byte, sourceIP net.IP, nasID string, timeout time.Duration) bool {
	if len(secret) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := radius.NewClient(radius.ClientConfig{Timeout: timeout, Retries: 1, SourceAddress: sourceIP})
	if err != nil {
		return false
	}
	defer func() { _ = client.Close() }()

	auth, err := radius.RandomAuthenticator()
	if err != nil {
		return false
	}
	attrs := []radius.Attr{
		{Type: radius.AttrUserName, Value: radius.AttrString("ze-doctor")},
		{Type: radius.AttrUserPassword, Value: radius.AttrString("ze-doctor")},
	}
	if nasID != "" {
		attrs = append(attrs, radius.Attr{Type: radius.AttrNASIdentifier, Value: radius.AttrString(nasID)})
	}
	pkt := &radius.Packet{
		Code:          radius.CodeAccessRequest,
		Identifier:    byte(time.Now().UnixNano()),
		Authenticator: auth,
		Attrs:         attrs,
	}
	_, err = client.Exchange(ctx, pkt, secret, addr)
	return err == nil
}

// radiusAuthContainer walks l2tp > auth > radius, returning nil when any
// level of the block is absent.
func radiusAuthContainer(tree *config.Tree) *config.Tree {
	cur := tree
	for _, name := range []string{"l2tp", "auth", "radius"} {
		if cur == nil {
			return nil
		}
		cur = cur.GetContainer(name)
	}
	return cur
}

// doctorTimeout reads a positive integer leaf as seconds, falling back to def.
func doctorTimeout(tree *config.Tree, leaf string, def int) time.Duration {
	if v, ok := tree.Get(leaf); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(def) * time.Second
}

// doctorValueOrDefault reads a leaf value, falling back to def when absent or empty.
func doctorValueOrDefault(tree *config.Tree, name, def string) string {
	if tree == nil {
		return def
	}
	if v, ok := tree.Get(name); ok && v != "" {
		return v
	}
	return def
}
