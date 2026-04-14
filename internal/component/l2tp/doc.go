// Package l2tp implements L2TPv2 (RFC 2661) wire format parsing and
// serialization for ze's L2TP subsystem.
//
// Phase 1 scope (this file set): header, AVPs, challenge/response, hidden
// AVP encryption. No network I/O, no state machines, no kernel interaction.
// Consumed by later phases (reactor, tunnel FSM, session FSM).
//
// Buffer discipline: parsing returns views into caller-owned byte slices;
// encoding writes into caller-provided buffers. No append in encoding
// helpers, no make in hot paths. See .claude/rules/buffer-first.md.
//
// Reference: rfc/short/rfc2661.md, docs/research/l2tpv2-implementation-guide.md,
// docs/research/l2tpv2-ze-integration.md.
package l2tp
