// Package ppp implements the PPP control plane (LCP, authentication,
// IPCP, IPv6CP) over chan/unit file descriptors obtained from
// /dev/ppp. It is transport-agnostic: callers (today: L2TP via
// internal/component/l2tp; in future: PPPoE) hand the manager a fresh
// pair of fds plus session metadata, and the manager drives PPP
// negotiation to "user up" before emitting events back.
//
// The package is a peer of internal/component/l2tp and MUST NOT import
// it. Coupling between the two flows in only one direction: l2tp imports
// ppp at the manager wiring point.
//
// Concurrency model: one long-lived goroutine per active PPP session,
// blocking reads on the chan fd via Go's runtime poller. A small
// manager-level lock guards the sessions map and per-session state for
// CLI introspection.
//
// Reference: docs/research/l2tpv2-ze-integration.md (PPP boundary),
// docs/research/l2tpv2-implementation-guide.md sections 22, Appendix C
// (PPP gap), RFC 1661 (LCP), RFC 1332 (IPCP), RFC 5072 (IPv6CP),
// RFC 1334/1994/2759 (PAP/CHAP/MS-CHAPv2).
package ppp
