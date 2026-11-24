// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass entry chooses HTTPS or HTTP, one rule for start and reload
//
// Always on, although the looking glass itself is tagged (ze_lg). The start
// path (service_lg.go) and the reload path (listener_migrate.go, main_reload.go)
// both ask this file which listeners a config describes and which protocol each
// one serves, so a restart and a reload cannot disagree about one config.
//
// Related: service_lg.go -- buildLGService applies the plan at start
// Related: listener_migrate.go -- updateLGTransport applies it at reload
// Related: service_tls.go -- listenerTLSMaterial, the certificate a TLS entry presents

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
)

// lgDefaultListen is the endpoint of a looking glass no config names an address
// for: the one extractServerList synthesizes for a block with no server entry.
const lgDefaultListen = "0.0.0.0:8443"

// lgListener is one looking-glass listener a config describes: the address it
// binds, and what its server entry says about TLS.
type lgListener struct {
	addr string
	tls  zeconfig.LGServerTLS
}

// lgListenerConfig is what one config asks of the looking-glass listeners: the
// listeners, and the certificate the TLS ones present. An empty certName is the
// self-signed pair, as it is for listenerTLSMaterial.
type lgListenerConfig struct {
	// listeners is empty when the config starts no looking glass. A running
	// one then keeps its listeners and their protocol.
	listeners []lgListener
	certName  string
}

// lgListenerConfigFrom reads what tree asks of the looking-glass listeners. The
// startup path and the reload path both call it, and so does the read of the
// running config a rollback returns to (runningListenerCertificates).
func lgListenerConfigFrom(tree *zeconfig.Tree) (lgListenerConfig, error) {
	listeners, err := resolveLGListeners(tree)
	if err != nil {
		return lgListenerConfig{}, err
	}
	return lgListenerConfig{listeners: listeners, certName: lgCertificateName(tree, listeners)}, nil
}

// resolveLGListeners answers the listeners the looking glass binds under tree,
// in the precedence the daemon has always used for the addresses:
// ze.looking-glass.listen, then a block that says `enabled true`, then
// ze.looking-glass.enabled with the addresses of a block that does not, then
// the default endpoint. No answer means nothing starts a looking glass.
//
// The protocol of a listener is the tls leaf of its server entry. An address
// that comes from ze.looking-glass.listen, and the default endpoint, have no
// entry: they take the leaf's default, HTTPS, as an entry that left it unset
// does. No environment variable chooses the protocol, so a deployment that
// wants HTTP there writes the server entry with `tls false`.
func resolveLGListeners(tree *zeconfig.Tree) ([]lgListener, error) {
	if listen := env.Get("ze.looking-glass.listen"); listen != "" {
		endpoints, err := zeconfig.ParseCompoundListen(listen)
		if err != nil {
			return nil, lgListenUnreadable(listen, err)
		}
		listeners := make([]lgListener, 0, len(endpoints))
		for _, endpoint := range endpoints {
			listeners = append(listeners, lgListener{addr: endpoint.String(), tls: zeconfig.LGServerTLSInherited})
		}
		return listeners, nil
	}
	// ADDRESSES come from a block that asks for a listener, so `enabled false`
	// still means "config does not start the looking glass".
	if cfg, ok := zeconfig.ExtractLGConfig(tree); ok {
		return lgListenersOf(cfg.Servers), nil
	}
	// ze.looking-glass.enabled says START, and says nothing about WHERE: a
	// dormant block still names the addresses, so an env-started looking glass
	// binds what the operator wrote (same defect as environment.gnmi,
	// cmd/ze/hub/gnmi_infra.go).
	if !env.IsEnabled("ze.looking-glass.enabled") {
		return nil, nil
	}
	if cfg, ok := zeconfig.ExtractLGSettings(tree); ok {
		return lgListenersOf(cfg.Servers), nil
	}
	return []lgListener{{addr: lgDefaultListen, tls: zeconfig.LGServerTLSInherited}}, nil
}

// lgListenersOf lowers the server entries of a looking-glass block to listeners.
func lgListenersOf(servers []zeconfig.LGServerEndpoint) []lgListener {
	listeners := make([]lgListener, 0, len(servers))
	for _, server := range servers {
		listeners = append(listeners, lgListener{addr: server.Listen(), tls: server.TLS})
	}
	return listeners
}

// lgListenerAddrs returns the addresses of listeners, in order.
func lgListenerAddrs(listeners []lgListener) []string {
	addrs := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		addrs = append(addrs, listener.addr)
	}
	return addrs
}

// lgAsksForTLS reports whether one listener's entry asks for TLS: every entry
// but one whose operator wrote `tls false`.
func lgAsksForTLS(listeners []lgListener) bool {
	for _, listener := range listeners {
		if listener.tls != zeconfig.LGServerTLSOff {
			return true
		}
	}
	return false
}

// lgBinding is one listener with the protocol the plan gives it.
type lgBinding struct {
	addr string
	tls  bool
}

// lgTransportPlan is the protocol each described listener serves on this
// daemon, and the listeners that serve HTTP only because TLS has nothing to
// present.
type lgTransportPlan struct {
	bindings []lgBinding
	// downgraded holds the addresses whose entry left tls unset and that serve
	// HTTP for want of a certificate. Each one owes the operator a warning.
	downgraded []string
}

// servesTLS reports whether one binding of the plan serves TLS, which is when
// the looking glass needs a certificate.
func (p lgTransportPlan) servesTLS() bool {
	for _, binding := range p.bindings {
		if binding.tls {
			return true
		}
	}
	return false
}

// tlsByAddr returns the plan keyed by address, the form the running looking
// glass takes it in (lgTransportUpdatable).
func (p lgTransportPlan) tlsByAddr() map[string]bool {
	out := make(map[string]bool, len(p.bindings))
	for _, binding := range p.bindings {
		out[binding.addr] = binding.tls
	}
	return out
}

// planLGTransports decides the protocol of every listener a config describes.
// It is the one place the rule is written, and start and reload both call it.
//
// An entry that says `tls false` serves HTTP. Any other entry serves TLS when
// there is a certificate to present: a named one, which comes from the pki
// container, or the self-signed pair, which needs the database store
// (hasStore). With neither:
//
//   - an entry that left tls unset serves HTTP, and is reported in downgraded:
//     the default must not turn a working looking glass into a missing one;
//   - an entry whose operator wrote `tls true` is refused, because that
//     instruction cannot be met and is never approximated.
func planLGTransports(listeners []lgListener, certName string, hasStore bool) (lgTransportPlan, error) {
	hasCertificate := hasStore
	if certName != "" {
		hasCertificate = true
	}

	plan := lgTransportPlan{bindings: make([]lgBinding, 0, len(listeners))}
	for _, listener := range listeners {
		serveTLS := false
		switch listener.tls {
		case zeconfig.LGServerTLSOff:
		case zeconfig.LGServerTLSRequired:
			if !hasCertificate {
				return lgTransportPlan{}, lgTLSNeedsCertificate(listener.addr)
			}
			serveTLS = true
		case zeconfig.LGServerTLSInherited:
			serveTLS = hasCertificate
			if !hasCertificate {
				plan.downgraded = append(plan.downgraded, listener.addr)
			}
		case zeconfig.LGServerTLSUnspecified:
			panic("BUG: looking-glass listener " + listener.addr + " built with no tls state")
		}
		plan.bindings = append(plan.bindings, lgBinding{addr: listener.addr, tls: serveTLS})
	}
	return plan, nil
}

// lgTLSNeedsCertificate is the refusal of a server entry whose operator wrote
// `tls true` on a daemon that has no certificate to present there.
func lgTLSNeedsCertificate(addr string) error {
	return opererr.New(
		"The looking glass server "+addr+" cannot serve TLS.",
		"Its entry says `tls true`, no certificate is named, and the self-signed certificate is kept in the database store, which this daemon runs without.",
		"Name a pki certificate in `environment looking-glass certificate`, or run `ze init` and restart ze, or write `tls false` in that server entry to serve HTTP.",
	)
}

// lgDowngradeWarning is the warning of a listener whose entry left tls unset
// and that serves HTTP for want of a certificate (lgTransportPlan.downgraded).
// The start path prints it, the reload path logs it, and `ze doctor` reports
// it (lg_doctor.go), so the three say the same thing.
func lgDowngradeWarning(addr string) *opererr.Error {
	return opererr.New(
		"The looking glass server "+addr+" serves plain HTTP: its tls leaf is unset and TLS has no certificate to present.",
		"No certificate is named, and the self-signed certificate is kept in the database store, which this daemon runs without.",
		"Run `ze init` and restart ze to serve HTTPS, or write `tls false` in that server entry to serve HTTP on purpose.",
	)
}

// lgListenUnreadable is the refusal of a ze.looking-glass.listen value that
// does not parse: the looking glass cannot tell which addresses to bind.
func lgListenUnreadable(listen string, cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"The environment variable ze.looking-glass.listen holds "+listen+", which is not a list of listen endpoints.",
		"The looking glass binds the addresses this variable names, so it cannot start or move its listeners",
		"Set ze.looking-glass.listen to endpoints such as 0.0.0.0:8443, or unset it and name the addresses in `environment looking-glass server`.",
	)
}
