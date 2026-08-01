# 734 -- IPsec Data Model

## Context

Ze needed a typed data model for IPsec site-to-site VPN configuration to feed into
a future strongSwan integration layer. No IPsec config existed; the goal was YANG schema,
Go types with algorithm enums, a tree-walker config parser, and cross-reference validation
against the PKI store and interface list.

## Decisions

- Followed the L2TP `ExtractParameters` tree-walker pattern over a code-gen approach because
  the existing config infrastructure (config.Tree, GetContainer, GetListOrdered) already
  handles YANG-to-Go extraction cleanly.
- Algorithm enum String() values match strongSwan swanctl.conf naming (e.g., `aes128gcm` not
  `aes-128-gcm`) over YANG kebab-case convention, because spec ipsec-4 needs direct mapping
  to swanctl.conf proposals.
- DHGroup as `uint8` with range validation (1-31) over an iota enum, because DH groups are
  identified by number in IKEv2 (RFC 7296 Section 3.3.2) and new groups should not require
  code changes.
- Cross-reference validation via function parameters (`func(string) bool`) over direct
  `pki` package import, keeping the ipsec package decoupled from pki for testability.
- `IPsecConfig.Changed()` compares peers field-by-field over `reflect.DeepEqual` to avoid
  reflection overhead and to make comparison criteria explicit.

## Consequences

- Spec ipsec-4 (strongSwan integration) can consume the typed structs directly to generate
  swanctl.conf. The algorithm enum String() values are the strongSwan proposal names.
- Adding new algorithms requires only adding an enum constant and a map entry in types.go;
  no parser changes needed.
- The `ValidatePKIRefs` function signature accepts callbacks, so production code passes
  `pki.GetCA`/`pki.GetCertificate`/`pki.CertCN` while tests use stubs.

## Gotchas

- The `gocritic` linter flags `uint32(n) > maxLifetime` as truncation; must cast the constant
  to uint64 instead: `n > uint64(maxLifetime)`.
- `gosec` G101 false-positive on `authModeNames` map containing "pre-shared-secret" as a
  string value; requires `//nolint:gosec` annotation.
- `gocritic` `rangeValCopy` triggers on map iteration of `SiteToSitePeer` (192 bytes);
  use `for name := range m` + indexed access instead of `for name, peer := range m`.
- `nilnil` linter rejects `return nil, nil` for `(*IPsecConfig, error)`; return an empty
  struct instead via a helper function.

## Files

- `internal/component/ike/ipsec/types.go` -- algorithm enums, struct types, Changed()
- `internal/component/ike/ipsec/config.go` -- tree-walker parser
- `internal/component/ike/ipsec/validate.go` -- cross-reference validation
- `internal/component/ipsec/register.go` -- blank import for schema registration
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` -- YANG module
- `internal/component/ike/ipsec/yang/embed.go` -- go:embed
- `internal/component/ike/ipsec/yang/register.go` -- init() yang.RegisterModule
- `internal/component/ike/ipsec/types_test.go` -- enum round-trip tests
- `internal/component/ike/ipsec/config_test.go` -- parser and validation tests
- `cmd/ze/hub/main.go` -- blank import of ipsec package
- `docs/features.md` -- IPsec data model feature row
- `docs/guide/configuration.md` -- vpn { ipsec {} } config guide section
