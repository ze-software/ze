# Native IKEv2 and IPsec

> **Pre-Alpha.** This page describes behavior that may change.

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

Ze can act as the IKEv2 responder as well as the initiator. A peer with `connection-type respond` waits for an unsolicited inbound IKE_SA_INIT from its configured `remote-address`, answers IKE_AUTH, and installs the first Child SA. `connection-type initiate`, the default, starts the exchange. The UDP transport always listens, so responder mode does not need a separate listen switch.

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

For a road-warrior EAP server, set `authentication { mode eap-mschapv2 }` or `eap-tls`, then reference a device `certificate` and `ca-certificate` from the PKI store. The `remote-access` container assigns client addresses from a `pool` and holds per-user EAP credentials in an `eap-user` list.

When a CA certificate is configured, the EAP peer validates the authenticator's
certificate chain against that trust anchor. EAP-TLS has no server hostname, so
the check validates the chain without DNS-name matching. With no CA configured,
server-certificate validation is disabled explicitly.

<!-- source: internal/component/ike/eap/peer.go -- verifyServerChain, startTLSClient -->


## Rekeying

Both Child SAs and IKE SAs are rekeyed with an on-wire CREATE_CHILD_SA exchange. This replaced an earlier local-only key roll that could silently desynchronise a live tunnel. A Child SA rekey carries `N(REKEY_SA)` with fresh nonces and traffic selectors. An IKE SA rekey performs a fresh Diffie-Hellman exchange and resets the message-ID counters.

Rekeying is make-before-break: the replacement SA is installed before the old one is deleted, so forwarding does not pause. Simultaneous rekeys are resolved by the RFC 7296 nonce rule, where the lower nonce wins, so both ends converge on one SA. Ze rekeys at the ESP or IKE soft lifetime and retransmits a lost CREATE_CHILD_SA before tearing the tunnel down. Rekeys are counted by `ze_ipsec_rekey_total{peer}` and streamed as `child-rekey` events by `monitor vpn ipsec`.

## Child SA and dataplane

Child SAs define traffic selectors and the ESP proposal. Ze programs XFRM policies and states through netlink. Route-based IPsec uses XFRM interfaces.

| Feature | Detail |
|---|---|
| ESP proposals | AES-GCM-16 128/256 and AES-CBC with HMAC-SHA-256/384/512 |
| Traffic selectors | IPv4 and IPv6 CIDR prefixes |
| Connection | `connection-type initiate` starts the exchange; `connection-type respond` waits for the peer |
| Replay protection | Anti-replay window, default 32 |
| Lifetime | Time-based and byte-based rekeying thresholds |

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

Prometheus exposes `ze_ipsec_sa_count`, `ze_ipsec_tunnel_up{peer}`, and `ze_ipsec_rekey_total{peer}`.

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
