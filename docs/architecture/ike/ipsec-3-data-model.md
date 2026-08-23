# The IPsec configuration data model

The typed model between the YANG schema and the IKE engine: algorithm enums,
peer and proposal structs, a tree-walker parser, and cross-reference validation
against the PKI store and the interface list.

<!-- source: internal/component/ike/ipsec/types.go -- IPsecConfig, SiteToSitePeer, IKEGroup, ESPGroup, Changed -->
<!-- source: internal/component/ike/ipsec/config.go -- ParseIPsecConfig, parseSiteToSitePeer, parseIKEGroup, parseESPGroup -->
<!-- source: internal/component/ike/ipsec/validate.go -- ValidatePKIRefs, ValidateGroupRefs, ValidateInterfaceRef -->
<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- vpn ipsec schema -->

## Decisions

**A tree-walker parser, not code generation.** The existing config
infrastructure already extracts YANG into Go through `config.Tree`,
`GetContainer` and `GetListOrdered`. The L2TP parameter walker is the pattern
this follows.

**Algorithm `String()` values match swanctl naming.** The enum prints
`aes128gcm`, not `aes-128-gcm` and not the YANG kebab-case spelling. The names
are the proposal names an operator already knows from other IPsec stacks.

<!-- source: internal/component/ike/ipsec/types.go -- EncryptionAlgo.String, HashAlgo.String, ParseEncryptionAlgo -->

**The DH group is a `uint8` with a range check, not an enum.** RFC 7296 Section
3.3.2 identifies Diffie-Hellman groups by number, so a new group must not
require a code change. The range is 1 to 31.

<!-- source: internal/component/ike/ipsec/types.go -- DHGroup, ValidDHGroup -->
<!-- source: internal/component/ike/ipsec/algorithm_support.go -- DHGroupImplemented, SupportedDHGroupIDs -->

**Cross-reference validation takes callbacks, not a `pki` import.**
`ValidatePKIRefs` accepts `func(string) bool` predicates. Production passes
`pki.GetCA`, `pki.GetCertificate` and `pki.CertCN`; tests pass stubs. The ipsec
package stays decoupled from pki.

<!-- source: internal/component/pki/store.go -- GetCA, GetCertificate, CertCN -->

**`Changed()` compares the WHOLE value, through `SiteToSitePeer.Equal`.** An
explicit field list was tried first, on the reasoning that it states the criteria
and costs nothing at reload time. It cost a defect instead. The list here named
eight members and `peerConfigChanged` (engine/reconcile.go) named a DIFFERENT
eight, so the two disagreed about what "changed" meant, and `TrafficSelectors`,
`Mode` and `TransportRequired` were in neither. An operator narrowed a live
peer's traffic selectors, the commit succeeded, and the tunnel kept carrying the
prefix the edit removed.

`reflect.DeepEqual` compares every member, so a member added to the type is
compared on the day it is added. Reload is a cold path, because it runs when an
operator commits, so the reflection costs nothing that matters.
Subtracting a member is still allowed, and it has to be done by name with the
reason recorded on `Equal`: what is banned is a member being ignored because
nobody listed it.

<!-- source: internal/component/ike/ipsec/types.go -- SiteToSitePeer.Equal, AuthConfig.Equal -->

## Traps this code exists to avoid

The linters in this repository push back on four shapes in this package. Each
one has a fixed answer:

| Complaint | Answer |
|-----------|--------|
| `gocritic` reads `uint32(n) > maxLifetime` as a truncation | cast the constant instead: `n > uint64(maxLifetime)` |
| `gosec` G101 fires on an auth-mode name map holding `pre-shared-secret` | annotate the map, the value is a display name |
| `gocritic` `rangeValCopy` on a map of peers | iterate `for name := range m` and index |
| `nilnil` rejects `return nil, nil` for a config pointer and an error | return an empty config from a helper |
