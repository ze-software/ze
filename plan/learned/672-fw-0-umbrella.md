# 672 -- fw-0-umbrella Firewall and Traffic Control Architecture

## Context

Ze needed native firewall (nftables) and traffic control (tc) replacing VyOS
firewall dependency. The umbrella defined scope, architecture, and ordering for
11 child specs (fw-1 through fw-10 plus fw-7b) delivering two new components,
four backend plugins, YANG config, CLI, and 18 functional tests.

## Decisions

- **Abstract data model over nftables-native.** 42 expression types (18 match,
  16 action, 8 modifier) model firewall concepts, not nftables register operations.
  The nft backend lowers to register chains internally; the VPP backend maps
  concepts directly to ACL fields. This was the foundational decision and it
  held up across all 11 child specs.
- **Backend interface pattern from iface.** RegisterBackend/LoadBackend/GetBackend
  with mutex-protected map. Proved extensible: Verifier infrastructure (fw-6)
  and context parameter (fw-7b) added without breaking the pattern.
- **Hybrid Junos/ze config syntax.** Named terms with from/then split, ze
  readable names, nftables concepts (table/chain/hook/set/flowtable). Structural
  safety prevents mis-ordering.
- **ze_* table ownership.** Plugin manages only ze_* tables, never touches
  non-ze_* tables. Lachesis coexistence validated by functional test.
- **VPP scope narrowed per exact-or-reject.** ACL-only for firewall (NAT44,
  classifier, policer deferred), single-class for traffic (multi-class needs
  VPP classify pipeline). Verifiers reject unsupported types at commit.
- **Plugin path convention.** Nested under domain (`firewall/nft/`, `firewall/vpp/`,
  `traffic/netlink/`, `traffic/vpp/`) not flat. Matches codebase convention.

## Consequences

- Ze has production-ready nftables firewall and tc traffic control on Linux,
  with VPP backends for ACL and single-class policer.
- The abstract data model boundary is validated: both nft and VPP backends
  consume the same types without leaking backend-specific concepts.
- The commit-time verifier pattern (RegisterVerifier/RunVerifier) is reusable
  for any future backend that cannot represent all expression types.
- Real VPP integration tests remain deferred to VPP CI infrastructure spec.

## Gotchas

- **Plugin paths diverged from spec early.** Original plan used flat names
  (`firewallvpp/`); codebase convention is nested (`firewall/vpp/`). Every
  child spec after fw-2 used the correct path, but the umbrella kept stale
  references until closure.
- **VPP multi-policer stacking.** VPP's output feature arc runs all policers
  in series, so N policers become min(rates). Caught in fw-7 review. The fix
  (restrict to single class) is correct but non-obvious.
- **VPP ACLInterfaceSetACLList replaces full list.** Input and output ACLs
  must be merged into a single vector with nInput boundary. Separate per-direction
  calls overwrite each other. Read-merge-write pattern required.
- **Backend.Apply gained context in fw-7b.** Signature change rippled to all
  backends. sdk.SignalContext() wired to 41 plugin runEngines.
- **fw-10 and fw-7b were unplanned.** Bug fixes and hardening needs emerged
  during implementation. Both were worthwhile additions.

## Files

### Components (33 Go files)
- `internal/component/firewall/` (19 files): model, backend, verifier, config, CLI, reactor, YANG schema
- `internal/component/traffic/` (14 files): model, backend, config, CLI, reactor, YANG schema

### Plugins (44 Go files)
- `internal/plugins/firewall/nft/` (9 files): nftables backend
- `internal/plugins/firewall/vpp/` (13 files): VPP ACL backend
- `internal/plugins/traffic/netlink/` (9 files): tc backend
- `internal/plugins/traffic/vpp/` (13 files): VPP policer backend

### Tests (18 .ci files)
- `test/firewall/` (14 .ci files)
- `test/traffic/` (4 .ci files)

### Child Learned Summaries
- 584-fw-1, 585-fw-4, 586-fw-2, 587-fw-3, 588-fw-5
- 623-fw-9, 627-fw-7, 629-fw-7b, 635-fw-10, 670-fw-8, 671-fw-6
