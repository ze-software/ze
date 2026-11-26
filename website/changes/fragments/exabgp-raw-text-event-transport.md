---
spec: exabgp-raw-text-event-transport
topic: ExaBGP Migration
---

## Before

Migrated ExaBGP scripts running through `ze exabgp plugin` received BGP UPDATE
events in Ze's `full` JSON format. For every UPDATE, the daemon decoded all
attributes and NLRIs into JSON alongside hex wire bytes, re-encoded each
external event batch through generic JSON reflection, and each wrapper process
parsed that JSON only to re-read wire attributes and serialize ExaBGP output
back out. On multi-script full-table feeds, this multiplied CPU and memory
overhead across every script process. Separately, the `bgp plugin format base64`
option was declared in the schema and command handler without a dedicated UPDATE
wire formatter.

## After

`bgp plugin format base64` now emits BGP UPDATEs as quote-free positional
counted ASCII lines carrying session metadata flags and the raw UPDATE body in
standard base64, while low-frequency control and session events (`state`,
`negotiated`, `open`, `notification`, `keepalive`, `refresh`) remain JSON with
`raw.message` wire bytes attached for packet and consolidated forms.

- In BGP-enabled builds, `ze exabgp plugin` automatically opts into `base64`
  delivery at startup with no operator configuration change.
- External TLS event batches now use pooled frame buffers with in-order batch
  splitting at the RPC frame size limit instead of per-batch JSON reflection.
- Inside `ze exabgp plugin`, hot IPv4 and IPv6 unicast UPDATEs (including
  ADD-PATH, End-of-RIB, and `--form packets|consolidate`) render directly from
  wire bytes into a byte buffer for ExaBGP 4.2/5.0/6.0 JSON and text output,
  while non-unicast families and malformed/unsplit NLRI cases reuse the
  wire-backed event renderer for identical script output.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
