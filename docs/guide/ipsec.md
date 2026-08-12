# Native IKEv2 and IPsec

> **Pre-Alpha.** This page describes behaviour that may change.

Ze implements native IKEv2 in Go for route-based IPsec VPN tunnels. It does not require strongSwan, libreswan, or another external IKE daemon. The IKE engine, cryptographic primitives, wire codec, and XFRM dataplane integration are all in-tree. The Ze binary negotiates IKE SAs, installs XFRM policies and states, and programs routes through XFRM interfaces from the same YANG config tree as every other subsystem.

## Architecture

The IPsec stack is split across several packages:

| Package | Role |
|---|---|
| `internal/component/ike/wire` | IKEv2 wire format codec for RFC 7296 payloads |
| `internal/component/ike/crypto` | DH groups, PRFs, integrity, encryption, and key derivation |
| `internal/component/ike/transport` | UDP transport with NAT-T keepalives and port 4500 encapsulation |
| `internal/component/ike/eap` | EAP-MSCHAPv2 and EAP-TLS authentication |
| `internal/component/ike/engine` | IKE_SA_INIT, IKE_AUTH, CREATE_CHILD_SA, and INFORMATIONAL state machines for initiator and responder roles, rekeying, and DPD |
| `internal/component/ike/ipsec` | YANG schema, configuration, and validation |
| `internal/component/ike/dataplane` | XFRM policy and state programming through netlink |
| `internal/component/pki` | X.509 certificate and private-key store |

## Configuration

IPsec is configured under `vpn { ipsec { } }`. Named `ike-group` and `esp-group` blocks hold the IKE and ESP proposals and lifetimes. Each `site-to-site` peer references a group pair, sets its authentication, and binds to an XFRM interface for route-based forwarding.

```text
pki {
    certificate my-cert {
        certificate-file /etc/ze/certs/router.pem;
        private-key-file /etc/ze/certs/router-key.pem;
    }
    ca-certificate my-ca {
        certificate-file /etc/ze/certs/ca.pem;
    }
}

vpn {
    ipsec {
        interface eth0;

        ike-group site-b-ike {
            key-exchange ikev2;
            lifetime     28800;
            proposal 1 {
                encryption aes256gcm;
                hash       sha256;
                dh-group   14;
            }
            dead-peer-detection { interval 30; timeout 120; }
        }

        esp-group site-b-esp {
            lifetime 3600;
            pfs      enable;
            proposal 1 { encryption aes256gcm; }
        }

        site-to-site {
            peer site-b {
                ike-group       site-b-ike;
                esp-group       site-b-esp;
                connection-type initiate;
                local-address   198.51.100.2;
                remote-address  198.51.100.1;
                authentication {
                    mode           x509;
                    certificate    my-cert;
                    ca-certificate my-ca;
                }
                vti { bind vti0; }
            }
        }
    }
}
```

Traffic selectors are not listed per tunnel. Route-based IPsec encrypts traffic that the routing table forwards through the bound XFRM interface. The generated [configuration reference](../../config-reference/#xfrm-interfaces) documents the XFRM interface leaves.

## IKE features

| Feature | Detail |
|---|---|
| IKEv2, RFC 7296 | Full IKE_SA_INIT, IKE_AUTH, CREATE_CHILD_SA, and INFORMATIONAL exchange support |
| Role | Initiator or responder per peer through `connection-type initiate` or `respond` |
| Proposals | AES-CBC, AES-GCM 128/256, and ChaCha20-Poly1305; MODP 2048/3072/4096/8192 and ECP 256/384/521 DH groups; SHA-256/384/512 PRFs |
| Authentication | Pre-shared key, X.509 certificates, EAP-MSCHAPv2, and EAP-TLS; Ze acts as the EAP authenticator in responder mode |
| NAT-T, RFC 3948 | Automatic NAT detection, UDP encapsulation on port 4500, and keepalives |
| DPD | Dead Peer Detection through INFORMATIONAL exchanges with configurable interval and timeout |
| Rekeying | On-wire CREATE_CHILD_SA rekeying for IKE and Child SAs, make-before-break installation, and collision handling |
| MOBIKE, RFC 4555 | Not implemented; an endpoint address change requires the SA to be re-established |
| Virtual IP pool | Remote-access server assignment from a configured pool |

## IKEv2 responder role

Ze can act as an IKEv2 responder or initiator. A peer with `connection-type respond` waits for an unsolicited inbound IKE_SA_INIT from its configured `remote-address`, answers IKE_AUTH, and installs the first Child SA. `connection-type initiate`, the default, starts the exchange. The UDP transport always listens, so responder mode does not need a separate listen switch.

As responder, Ze authenticates with a pre-shared key, X.509 certificate, or EAP. For EAP it acts as the EAP-MSCHAPv2 or EAP-TLS server for a road-warrior client. It presents its own certificate or PSK first, runs the EAP method, and derives session keys from the EAP MSK.

A new inbound IKE_SA_INIT can proceed while an older established SA for the
same peer is still being maintained. Once the replacement authenticates, its
INITIAL_CONTACT notification tells the remote endpoint to discard the stale SA.
This lets a responder recover immediately after an operator clear or peer
restart instead of waiting for Dead Peer Detection.

<!-- source: internal/component/ike/engine/established.go -- owned SA routing -->
<!-- source: internal/component/ike/engine/auth.go -- INITIAL_CONTACT -->


```text
vpn {
    ipsec {
        interface eth0;
        ike-group IKE-PSK {
            key-exchange ikev2;
            proposal 1 { encryption aes256gcm; hash sha256; dh-group 14; }
        }
        esp-group ESP-PSK {
            pfs enable;
            proposal 1 { encryption aes256gcm; }
        }
        site-to-site {
            peer swan {
                ike-group       IKE-PSK;
                esp-group       ESP-PSK;
                connection-type respond;
                local-address   172.28.0.2;
                remote-address  172.28.0.3;
                authentication {
                    mode              pre-shared-secret;
                    local-id          "172.28.0.2";
                    remote-id         "172.28.0.3";
                    pre-shared-secret "$9$encoded";
                }
            }
        }
    }
}
```

For a road-warrior EAP server, set `authentication { mode eap-mschapv2 }` or `eap-tls`, then reference a device `certificate` and `ca-certificate` from the PKI store. Configure the client as a `site-to-site` peer.

The `remote-access` container is not wired yet. It parses, and no session reads
it, so a `pool` assigns no address and an `eap-user` list authenticates nobody.
`plan/spec-ipsec-remote-access.md` owns the work.

<!-- source: internal/component/ike/engine/register.go -- runEngine discards the pool it builds from RemoteAccess -->

Every EAP mode and X.509 needs a `ca-certificate`. The daemon refuses a remote
certificate it cannot chain to that anchor. A certificate with no trust anchor
authenticates nobody: any self-signed certificate that carries a valid signature
would pass. RFC 7296 Section 2.16 requires EAP to run with a public-key
authentication of the responder, and the anchor is what makes that signature
attributable.

`ca-certificate` holds one anchor, not a path. A two-level authority therefore
needs the peer to send its intermediate certificates. RFC 7296 Section 3.6 puts
the peer certificate first, and Ze reads every later CERT payload as a link
toward the anchor. Ze sends its own chain the same way: the `certificate` entry
first, then the intermediate that PKI entry carries. A refusal names the anchor
and the number of intermediate certificates the peer supplied.

Ze refuses a config that names no `ca-certificate` at commit and at reload. It
does NOT refuse it under `ze config validate`, which reads the schema and does
not run a plugin's config verifier.

<!-- source: internal/component/ike/ipsec/validate.go -- ValidatePKIRefs -->
<!-- source: internal/component/ike/engine/config.go -- validateIPsecSections is the ike plugin's OnConfigVerify body -->
<!-- source: internal/component/ike/engine/auth.go -- getRemoteCert -->

## Remote identity

`remote-id` is the identity Ze expects the remote endpoint to assert. When it is
set, Ze enforces it twice on every authentication.

| Check | What it compares | Refusal |
|-------|------------------|---------|
| Policy | The IDi or IDr payload the peer sent against `remote-id` | Every authentication mode |
| Certificate | The same asserted identity against the remote certificate | Certificate modes only |

The certificate check is the half that matters when one authority issues to
several clients. The chain proves the authority issued the certificate. It does
not prove the certificate speaks for this peer, because the peer chooses the
identity its signature covers. Without the second check any client of that
authority authenticates as any peer.

A subject alternative name binds. The subject common name binds only when the
certificate carries no alternative name extension at all. The two fields do not
carry the same attestation: X.509 name constraints reach the alternative names
and never the subject distinguished name. An authority that permits only
`dNSName .branch.example.com` leaves the common name free, so a common name read
after a present alternative name would defeat that constraint.

The value you configure picks the field. An address `remote-id` binds against
the address alternative names alone, never against a domain name that spells the
same address. Certificate authorities issue an address alternative name under a
tighter policy than a name, and this stops the peer from choosing the weaker
field.

Ze compares six identity types: `ID_IPV4_ADDR`, `ID_IPV6_ADDR`, `ID_FQDN`,
`ID_RFC822_ADDR`, `ID_KEY_ID`, and `ID_DER_ASN1_DN`. A domain name and a mail
address compare with the ASCII letters folded. An address compares as an address,
so the encoding does not matter. A distinguished name compares in RFC 4514 string
form, and it binds against the certificate subject octet for octet. A peer that
asserts `ID_DER_ASN1_GN` is refused, because Ze compares no general name.

A `local-id` written as a distinguished name is still refused at commit. Ze
derives the type it SENDS from the shape of the value, and that derivation has no
`ID_DER_ASN1_DN` form.

RFC 7296 Section 4 requires that you can configure Ze to accept a PKIX peer whose
identity is `ID_KEY_ID`. An opaque key id matches no certificate field, so Ze
cannot derive that binding and denies it by default. Set `remote-id-type key-id`
to state that the chain to `ca-certificate` plus the exact key id IS the binding
you intend.

`remote-id-type` also pins ONE identity type for the peer. Without it a text
`remote-id` accepts `ID_FQDN`, `ID_RFC822_ADDR` and `ID_KEY_ID` alike. All three
compare as text. Set `remote-id-type rfc822-address` when you know the value is a
mail address. Ze then refuses a peer that asserts `ID_FQDN` with the same text.

The leaf governs what Ze ACCEPTS. The type Ze sends still follows `local-id`.

**An unset `remote-id` runs neither check.** Every certificate the configured
authority issued then authenticates as this peer. Ze logs a warning that names
the peer and the identity it accepted. Set `remote-id` whenever the authority
issues to more than one client.

<!-- source: internal/component/ike/engine/remote_id.go -- checkRemoteIdentity, certificateCarriesIdentity, assertedIdentity, hasSubjectAltName, configuredClass -->
<!-- source: internal/component/ike/engine/cert_payload.go -- getRemoteCert, storeRemoteCerts, buildCertPayloads -->
<!-- source: internal/component/ike/ipsec/validate.go -- ValidateIdentities -->

## Certificate chain length

`certificate-count` bounds the X.509 chain in both directions. It is the most
certificates Ze sends for the peer, and the most it accepts from it. RFC 7296
Section 3.6 sets that figure at four, and the default is four, so a peer you never
configure gets the conformant behaviour.

A peer that sends more than the bound is REFUSED. Ze does not truncate the chain.
A silent trim hides from you that the limit was reached. It also makes the
surviving certificates depend on the order the peer chose.

Name the intermediates on the PKI certificate entry, in order from the issuer of
your device certificate toward the trust anchor. Ze refuses at commit a peer whose
`certificate-count` is smaller than the chain its PKI entry holds.

<!-- source: internal/component/ike/engine/cert_payload.go -- storeRemoteCerts, localCertChain -->
<!-- source: internal/component/ike/ipsec/validate.go -- ValidateCertificateChains -->

## Hash and URL certificates

`hash-and-url` replaces the certificate on the wire with a 20-octet SHA-1 hash and
a URL that resolves to it (RFC 7296 Section 3.6). It keeps IKE messages short. Set
`certificate-url` to the http URL where you publish this device's certificate.

**The leaf defaults to false, and that default is a security property.** Resolving
a URL a peer sends makes an outbound request on behalf of a peer that is NOT yet
authenticated. Ze must fetch the certificate before it CAN authenticate the
peer. With the leaf false Ze advertises nothing, a conforming peer
sends no such payload, and Ze drops one from a peer that sends it anyway.

When you enable it, the lookup is bounded:

| Control | Bound |
|---------|-------|
| Scheme | `http` only. Every other scheme is refused before any name resolution |
| Response size | 64 KiB |
| Total timeout | 5 seconds |
| Redirects | None. A redirect is refused |
| Destination | Loopback, private, link-local, multicast and metadata addresses are denied |
| Hash | The SHA-1 is verified BEFORE any parser reads the fetched bytes |
| Cache | Keyed by the hash, never by the URL |

Use `certificate-url-allow` to permit a destination the deny list refuses. Run
`ze doctor` to check a configured `certificate-url`.

<!-- source: internal/component/ike/engine/certurl.go -- fetchHashAndURL, certURLDenied, certURLClient -->
<!-- source: internal/component/ike/engine/certbundle.go -- encodeCertBundle, decodeCertBundle -->
<!-- source: internal/component/ike/engine/doctor_certurl.go -- checkIPsecCertURL -->

The EAP peer validates the authenticator's certificate chain against that trust
anchor. EAP-TLS has no server hostname, so the check validates the chain without
DNS-name matching.

<!-- source: internal/component/ike/eap/peer.go -- verifyServerChain, startTLSClient -->


## Denial-of-service protection

RFC 7296 Section 2.6 names two attacks on an IKE responder: state and CPU exhaustion
from initiation requests with forged source addresses. The answer is a COOKIE. Ze
answers an inbound IKE_SA_INIT with a COOKIE notification and commits no state, and the
initiator must repeat its request with that cookie as the first payload before Ze
creates an SA or takes the peer's half-open handshake slot.

`cookie-threshold` sets how many half-open IKE SAs Ze tolerates before it challenges.
It defaults to `0`, which challenges every inbound initiation. That default costs a
genuine peer one extra round trip and closes a real availability hole: a responder peer
has one half-open slot, held for 30 seconds, so a single spoofed datagram bearing that
peer's source address would otherwise deny it service, and one datagram every 30 seconds
would deny it indefinitely. Raise the value to tolerate that many half-open handshakes
before challenging.

```text
vpn {
    ipsec {
        cookie-threshold 0;
    }
}
```

The cookie is an HMAC-SHA256 over the initiator's nonce, its source address and its SPI,
under a secret Ze generates and rotates every 60 seconds. A cookie minted under the
previous secret stays valid for one further interval, so a rotation never breaks an
in-flight handshake. A cookie that does not match is ignored rather than rejected, as
the RFC requires: the message is processed as though it carried no cookie, which for a
pressured responder means issuing a fresh challenge.

<!-- source: internal/component/ike/engine/cookie.go -- mintCookie, verifyCookie, cookieRequired -->
<!-- source: internal/component/ike/engine/register.go -- tryResponderSAInit, the gate before the half-open slot -->

## Diffie-Hellman group correction

An initiator has to guess which Diffie-Hellman group the responder will pick. When the
guess is wrong the responder answers INVALID_KE_PAYLOAD naming the group it accepts, and
Ze retries the IKE_SA_INIT under that group. Without this, Ze could never establish with
a peer that prefers a different group.

The retry re-offers the whole configured proposal set rather than narrowing it to the
suite carrying the responder's group. RFC 7296 Section 1.2 requires that, because the
rejection is unauthenticated and a narrowed re-offer would let an attacker who forges one
notify choose the cipher. Ze also refuses a group it never proposed, which stops the same
attacker choosing the group. Both COOKIE and INVALID_KE_PAYLOAD retries share one budget
of three per connection attempt, so two peers can never oscillate between them.

<!-- source: internal/component/ike/engine/sa_init_retry.go -- retrySAInit, parseInvalidKEGroup, groupIsProposed -->

## Rekeying

Both Child SAs and IKE SAs are rekeyed with an on-wire CREATE_CHILD_SA exchange. This replaced an earlier local-only key roll that could silently desynchronise a live tunnel. A Child SA rekey carries `N(REKEY_SA)` with fresh nonces and traffic selectors. An IKE SA rekey performs a fresh Diffie-Hellman exchange and resets the message-ID counters.

Rekeying is make-before-break: the replacement SA is installed before the old one is deleted, so forwarding does not pause. Simultaneous rekeys are resolved by the RFC 7296 nonce rule. The endpoint that created the SA with the lowest of the four nonces closes that SA, so both ends converge on one SA. Ze rekeys at the ESP or IKE soft lifetime and retransmits a lost CREATE_CHILD_SA before tearing the tunnel down. Rekeys are counted by `ze_ipsec_rekey_total{peer}` and streamed as `child-rekey` events by `monitor vpn ipsec`.

## Child SA and dataplane

Child SAs define traffic selectors and the ESP proposal. Ze programs XFRM policies and states through netlink. Route-based IPsec uses XFRM interfaces.

| Feature | Detail |
|---|---|
| ESP proposals | AES-GCM-16 128/256 and AES-CBC with HMAC-SHA-256/384/512 |
| Traffic selectors | IPv4 and IPv6 CIDR prefixes, an optional IP protocol, and an optional single port. See Traffic selectors and narrowing below |
| Encapsulation | Tunnel mode by default; `mode transport` negotiates transport mode per RFC 7296 Section 1.3.1 |
| Connection | `connection-type initiate` starts the exchange; `connection-type respond` waits for the peer |
| Replay protection | Anti-replay window, default 32 |
| Lifetime | Time-based and byte-based rekeying thresholds |

## Traffic selectors and narrowing

A peer's `traffic-selector` list states which traffic its Child SAs carry. It is also the
policy RFC 7296 Section 2.9 narrows a peer's proposal against.

```
vpn { ipsec { site-to-site { peer branch-1 {
    traffic-selector 1 {
        local  { prefix 10.1.0.0/16; }
        remote { prefix 10.2.0.0/16; }
    }
    traffic-selector 2 {
        protocol 6
        local  { prefix 10.3.0.0/24; port 179; }
        remote { prefix 10.4.0.0/24; }
    }
} } } }
```

<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- traffic-selector list -->

When the list is absent the peer accepts whatever the remote endpoint proposes. That is the
behaviour of every configuration written before the list existed, so adding the list is what
restricts a peer, never omitting it.

<!-- source: internal/component/ike/engine/ts_narrow.go -- narrowSelectors, the empty-policy branch -->

As responder, Ze narrows the initiator's proposed TSi and TSr to a subset the list allows,
and leads the answer with the initiator's first choices. A proposal it cannot narrow to a
non-empty subset draws a `TS_UNACCEPTABLE` notification. As initiator, Ze proposes the list
in order, because RFC 7296 reads the order as the preference order.

<!-- source: internal/component/ike/engine/ts_narrow.go -- narrowSelectors, narrowChildSelectors -->

The selectors Ze puts on the wire are the selectors it programs. A proposal it cannot
program exactly is narrowed FURTHER, never rounded outward: an address range that is not a
prefix becomes the largest prefix inside it, and a port range that is neither all ports nor
one port becomes its first port. A rekey is never narrowed below the scope in use.

<!-- source: internal/component/ike/engine/ts_narrow.go -- programmableSelector, largestPrefixIn, floorWithinProposal -->

| Port value | Meaning | RFC 7296 Section 3.13.1 encoding |
|---|---|---|
| `any` (the default) | every port | start 0, end 65535 |
| `1`..`65535` | one port | start N, end N |

A port other than `any` needs `protocol` to name a protocol that defines ports, such as 6
for TCP or 17 for UDP. Section 3.13.1 requires the port fields to be 0 and 65535 whenever
the protocol is 0, so the combination is refused at commit rather than widened silently.
Opaque ports are refused for the same reason: no dataplane backend can express an exact
match on port 0, so accepting one would install an any-port policy.

<!-- source: internal/component/ike/ipsec/traffic_selector.go -- checkPortProgrammable, PortSelector.Wire -->

## Transport mode

`mode transport` asks the peer for transport mode with the `USE_TRANSPORT_MODE`
notification of RFC 7296 Section 1.3.1. Tunnel mode is the default, and it is the RFC's own
default.

```
vpn { ipsec { site-to-site { peer host-1 {
    mode transport
    transport-required true
    traffic-selector 1 {
        local  { prefix 10.0.0.3/32; }
        remote { prefix 10.0.0.4/32; }
    }
} } } }
```

<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- mode, transport-required -->

Transport mode constrains the selectors. RFC 7296 Section 2.23.1 requires exactly one IP
address in TSi and in TSr, so every prefix must be a single host, and a `vti` binding is
refused because an XFRM interface carries tunnel encapsulation. Several selectors are still
allowed when they share that one address, for example to negotiate several ports.

<!-- source: internal/component/ike/engine/transport_mode.go -- transportSelectorPairs -->

A peer that declines the request establishes the Child SA in tunnel mode. Set
`transport-required true` when that downgrade is unacceptable: Ze then deletes the SA
instead, which is what Section 1.3.1 asks of an initiator. It defaults to false, so a peer
without transport mode keeps a working tunnel.

<!-- source: internal/component/ike/engine/transport_mode.go -- recordInitiatorTransportMode -->

The VPP dataplane backend does not implement transport mode. It refuses a transport-mode
install with a clear error rather than programming a tunnel-mode entry and reporting
success. The SA path and the policy path each refuse it on their own.

<!-- source: internal/component/ike/dataplane/vpp.go -- vppUnsupportedSA -->
<!-- source: internal/component/ike/dataplane/vpp_policy.go -- vppProtectMode -->

**The VPP dataplane backend cannot be driven by IKE, and no configuration selects it.**
It installs security associations, and it refuses every security policy the IKE engine
produces for them. VPP has no node-wide policy database: a policy lives in a security
policy database that acts only on the interfaces it is bound to, and nothing tells the
backend which interface to use. The backend is reached only through a private test
override, it is compiled in only with the `ze_vpp` build tag, and no test has sent ESP
through it. Use the XFRM backend, which is the default and the production path.

<!-- source: internal/component/ike/dataplane/vpp_policy.go -- vppPolicyInterface -->
<!-- source: internal/component/ike/engine/child.go -- childPolicyParams -->
<!-- source: internal/component/ike/engine/testport.go -- ikeDataplaneName -->

## PKI certificate store

The `pki { }` block stores X.509 certificates, private keys, and CA certificates. Certificates are loaded from PEM files and validated at commit time. The PKI store also serves TLS certificates for the web UI and gRPC API.

Health monitoring reports certificate expiry as a warning at 30 days and an error after expiry. Prometheus exposes `ze_pki_certificate_expiry_seconds` and `ze_pki_certificate_valid`.

## XFRM interfaces

XFRM interfaces provide route-based IPsec. Traffic routed through the XFRM interface is encrypted, and incoming traffic is decrypted before it appears on the interface. The generated [configuration reference](../../config-reference/#xfrm-interfaces) documents the interface surface.

## CLI

| Command | Description |
|---|---|
| `show vpn ipsec status` | Tunnel and peer status summary |
| `show vpn ipsec sa` | Active IKE and Child SAs, including algorithms, byte counts, rekey timers, SPIs, NAT detection, and initiator role |
| `show vpn ipsec peer name <name>` | Detail for one configured peer |
| `clear vpn ipsec sa [peer <name>]` | Tear down and re-establish SAs, optionally for one peer |
| `monitor vpn ipsec` | Stream `sa-up`, `sa-down`, `child-up`, `child-down`, and `child-rekey` lifecycle events |
| `show pki certificates` | List loaded certificates with expiry information |

`clear vpn ipsec sa` sends a best-effort encrypted IKE Delete before removing
local state. Initiator peers then re-establish immediately. If the UDP Delete is
lost, the normal DPD path still removes the stale remote SA.

<!-- source: internal/component/ike/engine/established.go -- graceful stop and sendDeleteIKE -->


## Health and metrics

The IPsec component registers with the health registry. It reports `healthy` when all configured tunnels are established, `degraded` when some are down, and `down` when critical tunnels fail.

Prometheus exposes `ze_ipsec_sa_count`, `ze_ipsec_tunnel_up{peer}`, `ze_ipsec_tunnel_degraded{peer}`, and `ze_ipsec_rekey_total{peer}`.

Three more counters report the COOKIE challenge described under [Denial-of-service protection](#denial-of-service-protection). `ze_ipsec_cookie_challenges_total{peer}` counts the challenges Ze issues, `ze_ipsec_cookie_verify_failures_total{peer}` counts inbound cookies that did not verify, and `ze_ipsec_sa_init_retries_total{peer,cause}` counts the IKE_SA_INIT retries Ze sends, labeled `cookie` or `invalid-ke-payload`. A rising verify-failure count is either an attacker probing the half-open slot or a secret rotation catching an in-flight challenge. A rising retry count on the `cookie` cause is the signature of the forged-notify flood RFC 7296 Section 2.6 describes.

`ze_ipsec_tunnel_up` reads 1 only when the IKE SA is established and the Child SA is
installed in the dataplane. A tunnel whose ESP install the kernel refused reads
`ze_ipsec_tunnel_up` 0 and `ze_ipsec_tunnel_degraded` 1. Such a tunnel has a live
control plane and carries no encrypted traffic. Alert on the degraded gauge, because
the two gauges together separate a lost session from a session with no ESP.

<!-- source: internal/component/ike/engine/metrics.go -- tunnelUp, tunnelDegraded, cookieChallenges, cookieVerifyFailures, saInitRetries -->
<!-- source: internal/component/ike/engine/child.go -- ChildSA.ESPInstalled -->

The daemon logs `child-sa: dataplane refused the ESP state, tunnel is degraded and
carries no encrypted traffic` when this happens.

## Interop testing

The IKE implementation includes interop tests against strongSwan 5.9.14, the version in the Alpine 3.21 test image. The test infrastructure in `test/ipsec-interop/` drives strongSwan containers as remote IKE peers. Coverage includes PSK, X.509, EAP-MSCHAPv2, and EAP-TLS authentication; Ze as a PSK and EAP-MSCHAPv2 responder against a strongSwan initiator; Child SA rekeying with make-before-break; and IKE SA rekeying initiated by strongSwan.

## See also

- The generated [configuration reference](../../config-reference/#ipsec-vpn) covers every IPsec and XFRM configuration leaf.
- [Monitoring](monitoring.md) covers the health registry and operational visibility.
- [Feature inventory](../features.md) shows the maturity of each IKE and IPsec capability.

<!-- source: internal/component/ike/engine/fsm.go -- IKE exchange state machine -->
<!-- source: internal/component/ike/engine/responder.go -- responder role -->
<!-- source: internal/component/ike/engine/rekey.go -- make-before-break rekeying -->
<!-- source: internal/component/ike/dataplane/dataplane.go -- XFRM state and policy programming -->
<!-- source: internal/component/ike/ipsec/config.go -- configuration and validation -->
<!-- source: test/ipsec-interop/run.py -- strongSwan interoperability scenarios -->
