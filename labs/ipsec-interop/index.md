Protocol lab

# IPsec / IKEv2 Interop

Ze as an IKE initiator against real strongSwan/charon, with FRR redistribute scenarios over the resulting tunnel.

`Daemon`

A peer-isolated Docker lab runs Ze as an IKEv2 initiator against strongSwan/charon as the responder, with FRR available as a BGP peer for redistribute scenarios over the established tunnel.

Unlike the L2TP and PPPoE labs, there's no long-form design document for this one yet -- the lab source itself (Docker lifecycle, PKI fixtures, scenario definitions) is the reference.

- **Proves:** IKEv2 negotiation and tunnel establishment against a real, independent IKE implementation
- **Peer:** Real strongSwan/charon (IKE responder), optional FRR for redistribute scenarios
- **Requires:** Docker, privileged containers
- **Source:** [test/interop-ipsec/](https://github.com/ze-software/ze/tree/main/test/interop-ipsec)

```
# all scenarios
$ make ze-interop-ipsec-test

# a single named scenario
$ make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=name
```

`Prerequisites`

Docker with privileged containers (IKE/IPsec needs kernel XFRM access).

- [test/interop-ipsec/ lab source](https://github.com/ze-software/ze/tree/main/test/interop-ipsec)
