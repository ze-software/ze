Protocol lab

# IPsec / IKEv2 Interop

Ze as an IKE initiator against real strongSwan/charon, with FRR redistribute scenarios over the resulting tunnel.

`Daemon`

A peer-isolated Docker lab runs Ze as an IKEv2 initiator against strongSwan/charon as the responder, with FRR available as a BGP peer for redistribute scenarios over the established tunnel.

The native Go runner owns Docker lifecycle, scenario discovery, and protocol checks. The checked-in lab tree supplies container definitions, PKI material, and scenario fixtures.

- **Proves:** IKEv2 negotiation and tunnel establishment against a real, independent IKE implementation
- **Peer:** Real strongSwan/charon (IKE responder), optional FRR for redistribute scenarios
- **Requires:** Docker, privileged containers
- **Source:** [internal/le/interoplab/ipsec/](https://github.com/ze-software/ze/tree/main/internal/le/interoplab/ipsec)

```
# all scenarios
$ ./le integration interop-ipsec

# a single named scenario
$ IPSEC_INTEROP_SCENARIO=name ./le integration interop-ipsec
```

`Prerequisites`

Docker with privileged containers (IKE/IPsec needs kernel XFRM access).

- [test/interop-ipsec/ scenario fixtures](https://github.com/ze-software/ze/tree/main/test/interop-ipsec)
