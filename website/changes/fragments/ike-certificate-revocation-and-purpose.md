---
spec: ike-certificate-revocation-and-purpose
topic: IPsec
feature: ikev2-engine
found-by: rfc-check
---

## Before

An IKEv2 peer authenticating with a certificate was accepted whatever its
revocation status: a `crl` on the `pki ca` entry was read by EAP-TLS only, so a
revoked peer certificate still established an IKE SA. The peer certificate's
key usage and extended key usage were not judged either, so a certificate
issued for TLS alone authenticated an IKE peer. A certificate revocation list
carrying a critical extension (a delta CRL indicator, an issuing distribution
point, a critical entry extension) was read as a complete list by EAP-TLS.

## After

IKEv2 checks every certificate on the peer's chain, except the trust anchor,
against the `crl` of each `pki ca` entry on that chain. Once any such entry
holds a list, a revoked certificate is refused, and so is one whose issuer has
no current list; the log names the certificate, its serial and the list's CRL
number. When no CA on the chain holds a list, the peer authenticates and a
warning says its revocation status was not checked. An intermediate CA is
added as a `pki ca` entry with its own `crl`.

A peer certificate must allow IKE, per RFC 4945: no extended key usage, or one
naming id-kp-ipsecIKE (1.3.6.1.5.5.7.3.17) or anyExtendedKeyUsage, and a key
usage, when present, with digitalSignature or nonRepudiation. A certificate
whose extended key usage is serverAuth only (a Let's Encrypt certificate, or a
strongSwan certificate issued with `--flag serverAuth --flag ikeIntermediate`)
is now refused, although strongSwan and libreswan accept it. Reissue it with
id-kp-ipsecIKE beside serverAuth.

A revocation list with a critical extension now answers nothing, for EAP-TLS
and IKEv2 alike, and the refusal names the extension.

## Corrects

none

## Exposure

Every IKEv2 peer using certificate authentication: a certificate the CA had
revoked, or one issued for another purpose by the same CA, authenticated as
long as it chained to `ca-certificate` and matched `remote-id`. Pre-shared key
and EAP peers that do not use a certificate were not exposed. EAP-TLS read a
delta or partitioned CRL as complete.

## Upstream

none

## Asked by

none
