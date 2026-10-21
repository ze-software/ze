# Learned: a receive rule and its sender frame alike, and a red is read from the image it runs

`spec-bgp-sr-policy-rfc-defects` closed three RFC 9830 defects in the SR Policy
SAFI: Ze sent a Binding SID label it was not configured with, judged no
received NLRI length, and accepted an SR Policy UPDATE with no Tunnel
Encapsulation attribute.

## What changed

- Origination: `parseConfigRoute` and `parseSegmentList`
  (`internal/component/bgp/plugins/nlri/srpolicy/config.go`) refuse a Binding
  SID label outside 16..1048575 and a Type A label above 1048575, naming the
  token, the value and the range, before any UPDATE is built.
- Receive: one per-family NLRI syntax registry,
  `internal/core/bgp/nlri/nlrisyntax`, consulted by `validateMPNLRISyntax`
  (`internal/component/bgp/message/rfc7606.go`) and by `typedNLRIEdit`
  (`internal/component/bgp/reactor/session_validation_nlritype.go`). SR Policy,
  BGP-LS and EVPN register their rules there; the hard-coded BGP-LS and EVPN
  branches are gone. A framed NLRI of the wrong length is treat-as-withdraw, an
  unwalkable section is a session reset with 3/1, and the synthesized
  withdrawal carries only the well-formed NLRIs.
- `applyTunnelEncap` (`internal/component/bgp/reactor/session_tunnel_encap.go`)
  treats a SAFI 73 MP_REACH with NLRI and no attribute 23 as treat-as-withdraw.
- Interop `bgp-sr-policy-gobgp` and `bgp-sr-policy-malformed-receive-gobgp`
  against GoBGP v3.31.0, green on Linux and in the macOS QEMU guest, with both
  revert-reds recorded.

## What outlives the spec

1. **A receive rule that honours a negotiated framing needs the sender to
   write it.** The SR Policy walk skipped a 4-octet ADD-PATH Path Identifier
   whenever ADD-PATH receive was negotiated, while Ze's own plugin-route rail
   never wrote one. Two Ze peers would have misjudged each other's every
   route. The fix framed in the shared layer (`message.PluginNLRI`,
   `sendPluginRoutesVia`), not per family, and the grouping key had to gain
   the Path Identifier so two routes that differ only in it never share an
   UPDATE.
2. **An expected red read from one version's source is a prediction about
   that version only.** The red for "Ze relays a 13-octet NLRI" was read from
   GoBGP v4.4.0 as a session reset. The scenario runs v3.31.0, which disables
   the AFI/SAFI and keeps the session, so the "session still up" assertion
   proved nothing and A's absence was the step that went red. Run the red
   against the pinned image before writing which assertion discriminates.
3. **A peer's API can corrupt what its wire decoder reads correctly.** GoBGP
   v3.31.0's apiutil divides the SR Policy NLRI length by 8 twice and shifts
   the Binding SID, so 1048575 read back as 1044480. The checker reads ListPath
   in binary and decodes with GoBGP's packet decoder
   (`plan/journal/field-carries-two-meanings.md`).
