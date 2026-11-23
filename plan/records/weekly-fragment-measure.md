# Measure: do news fragments carry the weekly update's facts

The weekly update needs five facts no commit message carries: what an
operator hit before the change (with what they see after), the earlier
ze-news claim it corrects, the configurations a security fix exposed, a bug
reported upstream, and who outside the project asked for it. News fragments
(`website/changes/fragments/README.md`) are meant to carry them. This file
records, for the next two weekly updates, where each fact of each published
item came from, so the owner can judge whether fragments replace the research
or whether their rules need tightening.

`/ze-weekly-update` appends one table per update once it is published
(`ai/skills/ze-weekly-update.md`, Phase 1, step 9). The table starts as the
`fragment-measure` section of `./le weekly report from <release-from> to
<release-to>`, which fills one row per fragment the range adds; the writer
marks what verification refuted or research found, and adds a row for each
published item with no fragment. Nothing here is published.

## Table shape

Each update adds a section headed by its range, whatever its length:

`## <release-from>..<release-to>`

followed by one row per item published in that update:

| Item | Before | Corrects | Exposure | Upstream | Asked by | Fragment |
|------|--------|----------|----------|----------|----------|----------|

- **Item:** the published item, in a few words.
- **Before** through **Asked by:** one of `fragment` (a fragment carried the
  fact and it passed verification at the producer), `research` (no fragment
  carried it, so the writer found it), `failed` (a fragment carried it and
  verification refuted it), or `n/a` (the item has no such fact). Before
  covers the Before and After headings together.
- **Fragment:** the fragment file's name, or `none` when the item had none.

Rows of `research` for a fact a fragment marked `none`, and items with no
fragment, are the signal that the fragment rules need tightening.

## 08788038a934df9969b6bf496997bcf3e4d347f1..7dcb14890bd695930a76bb8ef55b783af2b723ce

| Item | Before | Corrects | Exposure | Upstream | Asked by | Fragment |
|------|--------|----------|----------|----------|----------|----------|
| RPKI Invalid accepted by default | research | n/a | n/a | n/a | n/a | none |
| `next-hop auto` sets self towards eBGP | research | n/a | n/a | n/a | n/a | none |
| Unresolvable next hop not selected | research | n/a | n/a | n/a | n/a | none |
| Graceful restart syntax, bare block helper-only | research | research | n/a | n/a | n/a | bgp-llgr-per-family-config |
| Origin Validation State community dropped on eBGP | research | n/a | n/a | n/a | n/a | none |
| Old Ze syntax no longer rewritten | research | n/a | n/a | n/a | n/a | none |
| IPsec `unmatched` rewrite gone (corrects 12 October) | research | research | n/a | n/a | n/a | none |
| gNMI and MCP need TLS off loopback | research | n/a | research | n/a | n/a | none |
| IKEv2 refuses certificates without the IKE purpose | fragment | n/a | n/a | n/a | n/a | ike-certificate-revocation-and-purpose |
| IRR settings under `system irr` | fragment | n/a | n/a | n/a | n/a | fetched-data-survives-restart |
| Firewall prefix with host bits refused | fragment | n/a | n/a | n/a | n/a | firewall-prefix-normalize |
| Tunnel with unspecified remote refused | fragment | n/a | fragment | n/a | n/a | rfc-verdict-fix-services |
| Undeclared interface references refused | fragment | n/a | n/a | n/a | n/a | iface-mirror-target-reference |
| `show bgp` `group` key and own `description` | fragment | n/a | n/a | n/a | n/a | lg-peer-history-origin-search-and-named-nodes |
| RIB words are pipes | fragment | n/a | n/a | n/a | n/a | pipe-chain-honored-on-every-command-surface |
| SSH config editor ignored authorization profiles | fragment | n/a | fragment | n/a | n/a | ssh-editor-authorization |
| Appliance web UI `/gokrazy/` proxy | research | n/a | research | n/a | n/a | none |
| MCP identity scope not enforced | research | n/a | research | n/a | n/a | none |
| IPsec gateway leaked ports 500 and 4500 | fragment | n/a | fragment | n/a | n/a | rfc4301-architecture-gaps |
| IKEv2 revoked certificate accepted (corrects 7 September) | fragment | research | fragment | n/a | n/a | ike-certificate-revocation-and-purpose |
| IKEv2 forged handshake message | fragment | n/a | failed | n/a | n/a | ike-responder-eap-preserves-state-on-unauthenticated-input |
| IPsec on VPP inbound entries backwards | research | n/a | research | n/a | n/a | none |
| Fleet hub accepted an empty token | fragment | n/a | fragment | n/a | n/a | doctor-coverage-12-plugin-infra |
| PPPoE access concentrator on VPP | research | n/a | n/a | n/a | n/a | none |
| `clear l2tp session user` and `address` | research | n/a | n/a | n/a | n/a | none |
| RADIUS framed routes installed (corrects 11 May) | research | research | n/a | n/a | n/a | none |
| WireGuard peers survive a restart | research | n/a | n/a | n/a | n/a | none |
| BGP aggregation | research | n/a | n/a | n/a | n/a | none |
| Route selection waits for End-of-RIB after restart | research | n/a | n/a | n/a | n/a | none |
| Enhanced Route Refresh markers handled (corrects 5 January) | research | research | n/a | n/a | n/a | none |
| A peer that stops reading no longer holds its session | fragment | n/a | n/a | n/a | n/a | bgp-session-write-wedge |

Of 31 published items, 17 had no fragment, among them the four BGP default
changes, three of the security fixes and the whole access section. Every
published correction of an earlier post (four) came from research: no
fragment named one. One fragment's exposure was
stale, saying the initiator half was not fixed when it was. One fragment
(`docker-hosts-run-the-ze-kernel`) described work that shipped before the
range.
