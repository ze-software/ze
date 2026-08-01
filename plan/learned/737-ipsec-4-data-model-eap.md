# 737 -- IPsec Data Model EAP Extension

## Context

The IPsec data model (ipsec-3) covered IKE/ESP groups and site-to-site peers with
X.509 and PSK authentication. Road warrior VPN from Windows, macOS, iOS, and Android
clients requires EAP authentication (EAP-TLS and EAP-MSCHAPv2) and virtual IP pool
management. This spec extended the data model with those capabilities so that a
future IKEv2 engine (ipsec-9) can consume them.

## Decisions

- Extended the existing AuthMode enum with AuthEAPTLS and AuthEAPMSCHAPv2 over a
  separate EAP-specific enum, keeping one dispatch point in parseAuthConfig.
- EAP auth modes share the x509 certificate branch in parseAuthConfig (server needs
  certs for EAP too) over duplicating the x509 parsing logic.
- Single pool per remote-access config (first entry wins) over multi-pool, because
  strongSwan's swanctl.conf maps one pool per connection. YANG schema defines pool
  as a list for future extensibility.
- EAP passwords decoded from $9$ by the existing secret.Decode over adding a new
  encoding, matching the pattern for PSK, wireguard keys, and PPPoE passwords.
- RemoteAccessConfig as a pointer field on IPsecConfig over embedding, so nil
  clearly signals "not configured" and Changed() can detect add/remove.
- DNS stored as []string but parsed from a single YANG leaf (not leaf-list), because
  the current config tree API returns single values. Multiple DNS servers need a
  YANG schema change first.

## Consequences

- Spec ipsec-9 (IKEv2 EAP engine) can consume RemoteAccessConfig directly: pool
  ranges feed IKEv2 Configuration Payload, EAPUser entries feed the authenticator.
- ValidateRemoteAccess checks pool CIDR bounds (IPv4 /8-/30, IPv6 /48-/126) and
  credential completeness (password required for mschapv2, certificate for eap-tls).
- Changed() now detects remote-access config changes via the "remote-access" sentinel
  name, so a future config reload path can trigger strongSwan reconfiguration.

## Gotchas

- The gosec G117 linter flags the Password field name as a secret pattern; requires
  `json:"-"` tag and `//nolint:gosec` annotation.
- parseVirtualIPPool initially tried GetListOrdered("dns") for a YANG leaf (not list),
  which silently returned nothing. Caught in review. Always match parser method to
  YANG node type.
- The SA4004 linter rejects `for range list { break }` as an unconditionally terminated
  loop. Use `if len(list) > 0 { list[0] }` instead for single-element extraction.

## Files

- `internal/component/ike/ipsec/types.go` -- AuthEAPTLS/AuthEAPMSCHAPv2 enums, EAPUser, VirtualIPPool, RemoteAccessConfig, remoteAccessEqual
- `internal/component/ike/ipsec/config.go` -- parseRemoteAccess, parseVirtualIPPool, parseEAPUser
- `internal/component/ike/ipsec/validate.go` -- ValidateRemoteAccess, validatePoolPrefix
- `internal/component/ike/ipsec/config_test.go` -- EAP/pool parsing tests, validation tests, Changed() test
- `internal/component/ike/ipsec/types_test.go` -- extended AuthMode round-trip test
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` -- remote-access container, eap auth modes
- `test/parse/ipsec-remote-access.ci` -- functional test for remote-access config
- `test/parse/ipsec-eap-auth.ci` -- functional test for EAP-TLS config
