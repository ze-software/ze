# The IPsec configuration data model

The typed model between the YANG schema and the IKE engine: algorithm enums,
peer and proposal structs, a tree-walker parser, and cross-reference validation
against the PKI store and the interface list.

<!-- source: internal/component/ike/ipsec/types.go -- IPsecConfig, SiteToSitePeer, IKEGroup, ESPGroup, Changed -->
<!-- source: internal/component/ike/ipsec/config.go -- ParseIPsecConfig, parseSiteToSitePeer, parseIKEGroup, parseESPGroup -->
<!-- source: internal/component/ike/ipsec/validate.go -- ValidatePKIRefs, ValidateGroupRefs, ValidateInterfaceRef -->
<!-- source: internal/component/ike/ipsec/spd_policy.go -- SPDPolicy, parseSPDPolicy, ValidateSPDPolicies -->
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

**Algorithm admission follows the SA kind.** `EncryptionImplemented` and
`HashImplemented` retain the IKE cipher, integrity and PRF set.
`EncryptionImplementedESP` adds `chacha20poly1305`, `3des` and `null`, but refuses
AES-CCM because the dataplanes do not map it. `HashImplementedESP` accepts the
ESP integrity set, including legacy `sha1`, without requiring or admitting a
SHA-1 PRF. ESP AEAD proposals omit `hash`; non-AEAD proposals require it.
Error messages derive their supported names from the same registries.

<!-- source: internal/component/ike/ipsec/algorithm_support.go -- EncryptionImplemented, EncryptionImplementedESP, HashImplemented, HashImplementedESP -->


**An `ESPGroup` on a live Child SA holds ONE proposal, the accepted one.** The
configured group lists every proposal the operator offers, in order. After
negotiation the engine narrows the Child SA's own copy to the proposal the peer
selected, on both roles and on every rekey, so `ChildSA.ESPGroup.Proposals[0]` is
the installed transform and the configured list is never read for it. The
published Child SA state (`engine.PeerInfo`, and the `child-sa` object of `show
vpn ipsec sa`) reports that proposal and the three installed facts beside it that
size an ESP packet: the mode, the UDP encapsulation and the installed remote
endpoint. Reading the configured group there was the defect AC-15 of
spec-path-mtu-diagnostic removed.

<!-- source: internal/component/ike/engine/responder.go -- selectResponderESP -->
<!-- source: internal/component/ike/engine/reconcile.go -- PeerInfo, Info -->

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

**A remote-access pool is a named value, and a peer binds to it by name.**
`RemoteAccessConfig.Pools` maps each `remote-access pool` name to a
`VirtualIPPool` (its `range`, `range6` and `dns`). A `site-to-site` peer's
`virtual-ip-pool` leaf is `SiteToSitePeer.VirtualIP`, a `*VirtualIPBinding`
read through `VirtualIPPoolName`, nil for an unbound peer. It is a pointer
because `SiteToSitePeer` sits at gocritic's hugeParam edge
(`types_size_test.go`), and a 16-byte string did not fit. Commit refuses
overlapping pools, because both could lease one inner address to two peers
(RFC 3948 Section 5.1), an unknown pool name, the leaf on a peer whose
`connection-type` is not `respond`, traffic selectors whose remote prefixes do
not cover the pool, and a `domain` leaf, which Ze does not support. The engine builds the pools into one lease registry at
each apply, and its responder leases from them (`ipsec-14-responder.md`).

<!-- source: internal/component/ike/ipsec/types.go -- VirtualIPBinding, VirtualIPPoolName, VirtualIPPool, RemoteAccessConfig -->
<!-- source: internal/component/ike/ipsec/validate.go -- validateVirtualIPPools, validateVirtualIPPool, ValidateVirtualIPPoolBindings -->

**A second authentication round is one more `AuthConfig` per direction.**
`authentication second-round` holds a `local` and a `remote` container for the
RFC 4739 second round. `parseSecondRound` reads them into
`AuthConfig.Additions.SecondRound`, a `*SecondRoundAuth` whose `Local` and
`Remote` are each an `*AuthConfig`, nil when that container is absent. Readers go
through the nil-safe accessor `SecondRound()`, which answers nil when neither is
configured, and `HasSecondRound` answers whether either is. `Additions` is one
pointer shared with the intermediate certificates, which keeps `SiteToSitePeer`
under the 288-byte by-value threshold.
A round reuses the first round's fields rather than a second struct: `Local`
fills `Mode`, `PSK`, `LocalID` and `Certificate`, `Remote` fills `Mode`, `PSK`,
`RemoteID`, `RemoteIDType` and `CACertificate`, and each takes the peer's
`CertificateCount`. A round's mode is pre-shared-secret or x509, and the parser
refuses any other. The secret goes through `parsePreSharedSecret` and the PKI
name through `readCertificateRefs`, the first round's own readers.
`AuthConfig.Equal` compares the pointer deeply, so a second-round edit restarts
the peer as a first-round edit does.

At commit, `ValidateSecondRounds` refuses a second round on a peer whose first
round is an EAP mode, a pre-shared-secret round with no secret, a local x509
round with no certificate, a remote x509 round with no ca-certificate, and a
local round with no `id`: the round asserts an identity of its own, and a peer
that picks the round's pre-shared key by identity would otherwise look up the
first round's. It
checks the round's PKI names against the store and the local certificate's CN
against the round's `id`, as `ValidatePKIRefs` does for the first round. The
round ids pass through `validateIdentityLeaves`, the helper `ValidateIdentities`
runs over `local-id` and `remote-id`. `ValidateCertificateChains` holds the
round's certificate chain to the peer's `certificate-count`. Every refusal names
the peer and the leaf path, `authentication second-round local` or `remote`.

<!-- source: internal/component/ike/ipsec/types.go -- SecondRoundAuth, AuthConfig.HasSecondRound -->
<!-- source: internal/component/ike/ipsec/second_round.go -- parseSecondRound, ValidateSecondRounds -->
<!-- source: internal/component/ike/ipsec/validate.go -- validateIdentityLeaves, validateCertificateChain -->

## Traps this code exists to avoid

The linters in this repository push back on four shapes in this package. Each
one has a fixed answer:

| Complaint | Answer |
|-----------|--------|
| `gocritic` reads `uint32(n) > maxLifetime` as a truncation | cast the constant instead: `n > uint64(maxLifetime)` |
| `gosec` G101 fires on an auth-mode name map holding `pre-shared-secret` | annotate the map, the value is a display name |
| `gocritic` `rangeValCopy` on a map of peers | iterate `for name := range m` and index |
| `nilnil` rejects `return nil, nil` for a config pointer and an error | return an empty config from a helper |
