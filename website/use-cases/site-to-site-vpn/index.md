# Site-to-site IPsec and WireGuard VPN

Connect branch offices or cloud sites to a Linux router using Ze's native IKEv2 IPsec stack (programmed directly into Linux XFRM VTI interfaces) or kernel WireGuard interfaces, paired with static or dynamic routing through the tunnel interface.

## Role and topology

```text
Site A (Ze, 10.78.1.0/24)                  Site B (strongSwan or Ze, 10.78.2.0/24)
  172.30.0.1 ═══ IKEv2/ESP (XFRM vpn-xfrm) ═══ 172.30.0.2
  198.51.100.1 ═══ WireGuard (wg0)         ═══ 198.51.100.2
```

## Ze configuration: route-based IPsec IKEv2

Route-based IPsec binds an IKEv2 peer to an `xfrm` interface (`if-id 42`) and routes remote subnets into that interface (based on `demos/terminal/topics/ipsec/` and `test/interop-ipsec/scenarios/psk-site-to-site/ze.conf`):

```text
interface {
    xfrm vpn-xfrm {
        if-id 42;
        mtu 1400;
    }
}

static {
    table default {
        route 10.78.2.0/24 {
            next {
                interface vpn-xfrm {}
            }
        }
    }
}

vpn {
    ipsec {
        interface eth0;
        ike-group IKE-PSK {
            key-exchange ikev2;
            lifetime 28800;
            proposal 1 {
                encryption aes256gcm;
                hash sha256;
                dh-group 14;
            }
            dead-peer-detection { interval 30; timeout 120; }
        }
        esp-group ESP-PSK {
            lifetime 3600;
            pfs enable;
            proposal 1 {
                encryption aes256gcm;
            }
        }
        site-to-site {
            peer responder {
                ike-group IKE-PSK;
                esp-group ESP-PSK;
                connection-type initiate;
                mode tunnel;
                local-address 172.30.0.1;
                remote-address 172.30.0.2;
                vti {
                    bind vpn-xfrm;
                }
                traffic-selector 1 {
                    local { prefix 10.78.1.0/24; }
                    remote { prefix 10.78.2.0/24; }
                }
                authentication {
                    mode pre-shared-secret;
                    local-id "172.30.0.1";
                    remote-id "172.30.0.2";
                    pre-shared-secret "interop-test-psk";
                }
            }
        }
    }
}
```

X.509 certificate authentication uses `pki { ca ...; certificate ...; }` with `authentication { mode x509; ca-certificate ...; certificate ...; }` (see `test/interop-topology/cpe/ze-cpe.conf`).

## Ze configuration: WireGuard site-to-site

For lightweight peer-to-peer tunnels, declare a `wireguard` interface and point a static route at `wg0` (generate keys with `request wireguard keypair`; based on `test/reload/tx-iface-wireguard-apply.ci`):

```text
interface {
    wireguard wg0 {
        listen-port 51820;
        fwmark 0;
        private-key "$9$...";
        peer site2 {
            public-key "<base64-32-byte-pubkey>";
            preshared-key "$9$...";
            endpoint {
                ip 198.51.100.2;
                port 51820;
            }
            allowed-ips [ 10.0.0.2/32 192.168.10.0/24 ];
            persistent-keepalive 25;
        }
        unit 0 {
            ipv4 {
                address [ 10.0.0.1/24 ];
            }
        }
    }
}

static {
    table default {
        route 192.168.10.0/24 {
            next {
                interface wg0 {}
            }
        }
    }
}
```

## Adjacent peer configuration (strongSwan 6.1.0)

Responder configuration tested in `test/interop-ipsec/scenarios/psk-site-to-site/swanctl.conf`:

```ini
connections {
    ze {
        local_addrs = 172.30.0.2
        remote_addrs = 172.30.0.1
        local { auth = psk; id = 172.30.0.2 }
        remote { auth = psk }
        proposals = aes256gcm16-sha256-modp2048
        children {
            ze-child {
                local_ts = 0.0.0.0/0
                remote_ts = 0.0.0.0/0
                esp_proposals = aes256gcm16
                mode = tunnel
            }
        }
    }
}
secrets {
    ike-ze { secret = "interop-test-psk" }
}
```

## Verification commands

```text
show vpn ipsec status
show vpn ipsec sa
show vpn ipsec peer name responder
show vpn ipsec dataplane sa
show vpn ipsec dataplane policy
show vpn ipsec dataplane drift
show interface type wireguard
show static
```

`show vpn ipsec dataplane drift` compares the IKE engine's installed state against kernel XFRM SAs and policies and exits non-zero if they diverge.

## Lab evidence

- **strongSwan 6.1.0 interop (PSK, IPv6-in-IPv4 VTI, Child rekey, MOBIKE, BGP over IPsec, EAP-TLS 1.3):** `IPSEC_INTEROP_SCENARIO=psk-site-to-site ./le test integration interop-ipsec` and `IPSEC_INTEROP_SCENARIO=vti-ipv6-inner-ipv4-outer ./le test integration interop-ipsec` (`test/interop-ipsec/scenarios/`)
- **Linux kernel XFRM cipher proof (AES-GCM, AES-CBC/SHA256, ChaCha20-Poly1305):** `./le test qemu vpn-kernel`
- **X.509 management tunnel and WireGuard road-warrior topologies:** `./le test integration interop-topology scenario mgmt-tunnel` and `road-warrior`
- **Functional reload and dataplane checks:** `test/ipsec/ipsec-sa-installed.ci`, `test/ipsec/ipsec-child-rekey-xfrm.ci`, `test/reload/tx-iface-wireguard-apply.ci`

## Current limits

- **Experimental status:** Ze's native IPsec engine is currently marked Experimental / Pre-Alpha.
- **Linux kernel backend only for XFRM:** `interface { xfrm }` requires Linux with `backend kernel`. Establishing an IKE and Child SA does not install routes automatically; add a static route or run a routing protocol over the XFRM interface.
- **MOBIKE kernel requirement:** MOBIKE address migration requires Linux 7.2's `XFRM_MSG_MIGRATE_STATE` API (`CONFIG_XFRM_MIGRATE`).
- **WireGuard constraints:** `endpoint.ip` accepts a numeric IP address only (no DNS resolution). `$9$` values in configuration files are reversible obfuscation rather than encryption. VPP WireGuard device creation is not wired, and interface-only static next-hops (`next { interface wg0 {} }`) do not support BFD.

## Related guides

- [IPsec VPN guide](../../guides/ipsec/)
- [Interfaces](../../features/interfaces/)
- [IPsec interop lab](../../labs/ipsec-interop/)
