---
spec: eap-tls-identity-authorization
topic: IPsec
feature: ipsec-eap-authentication
found-by: rfc-check
---

## Before

Both ends of an IPsec EAP-TLS site-to-site tunnel validated the remote
certificate chain and its revocation status, but neither checked that the
identity carried in the certificate was authorized for that peer. Any valid,
unrevoked certificate signed by the configured `ca-certificate` could establish
a tunnel as a client or impersonate an EAP-TLS server.

Peer and server diagnostics also read only the certificate Subject DN instead
of the `subjectAltName` extension defined by RFC 5216 Section 5.2, and
completed EAP-TLS authentication logs did not record the verified peer or
server identity.

## After

Both EAP-TLS roles now extract Peer-Id and Server-Id from the certificate's
`subjectAltName` extension first (in wire order, falling back to Subject Common
Name only when no `subjectAltName` extension is present) and enforce identity
authorization on both full and resumed TLS 1.3 handshakes under RFC 5216
Section 5.3 and RFC 2818 Section 3.1.

- **Responder peers (`connection-type respond`):** Configure
  `authentication allowed-identities` with one or more authorized client
  identities: exact DNS names, single-label DNS wildcards (`*.branch.example.com`
  with at least two suffix labels), exact NAIs (`alice@example.com`), NAI realms
  (`@example.com`), exact IPv4/IPv6 addresses (matched only against `iPAddress`
  SANs), exact URIs, or a lone `*` to explicitly accept any valid CA-signed
  client certificate carrying a non-empty identity. Mixing `*` with specific
  entries is refused.
- **Initiator peers (`connection-type initiate`):** Configure
  `authentication server-identity` with an exact IP address, DNS hostname/FQDN,
  or `user@realm` NAI (no wildcards). When omitted, Ze derives the expected
  server identity from `remote-address` only when `remote-address` is an
  unambiguous multi-label FQDN; an IP literal, single-label name, or empty
  `remote-address` requires an explicit `server-identity`.
- **Fail-closed config and runtime checks:** `ze config validate`, commit, and
  reload refuse any `eap-tls` site-to-site peer missing its identity constraint,
  setting the opposite role's leaf, or setting either leaf on a non-`eap-tls`
  peer, and print the exact `set` or `delete` fix command. A daemon started from
  an unvalidated config file refuses unconstrained handshakes fail-closed and
  `ze doctor` flags affected peers under `doctor-ipsec-eap-tls-identity`.
- **Alerts, resumption, and logging:** An unauthorized client or mismatched
  server draws a fatal TLS `bad_certificate` alert and establishes no SA.
  Resumed TLS 1.3 sessions re-check identity against the cached leaf, and
  editing `allowed-identities` or `server-identity` restarts the peer and
  invalidates cached session tickets. Successful exchanges log `peer-id` on the
  responder or `server-id` on the initiator.

## Corrects

none

## Exposure

Any IPsec site-to-site peer configured with `authentication mode eap-tls` whose
`ca-certificate` trust anchor signs certificates for more than one client or
server was exposed: a holder of another valid, unrevoked certificate from the
same CA could authenticate as an unintended client (responder role) or server
(initiator role). Deployments using `pre-shared-secret`, `x509`, `eap-mschapv2`,
or `eap-md5`, or `eap-tls` peers whose trust anchor issued only a single
intended leaf certificate, were not exposed.

## Upstream

none

## Asked by

none
