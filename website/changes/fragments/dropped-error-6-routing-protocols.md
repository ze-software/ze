---
spec: dropped-error-6-routing-protocols
topic: Diagnostics
---

## Before

In OSPF, IS-IS, LDP and BFD, 28 failures were logged at debug or info, or not
at all, so an operator at the default log level never saw them:

- an OSPFv3 IPsec security association or policy Ze failed to delete stayed in
  the kernel, and Ze had already forgotten it;
- an IS-IS or LDP Hello that failed to send, until the neighbors dropped the
  adjacency at hold-time expiry;
- BFD packets that failed authentication, BFD packets the transport refused to
  send, and a meticulous-authentication sequence number the state store did
  not save, which after a restart keeps the session Down until the peer
  accepts the number again;
- an LDP session the neighbor ended, logged only at info as
  `ldp: session ended`;
- an event the event bus refused to publish, and a BFD session the BFD engine
  refused to release.

An LDP Hello whose message length was below 4 made the LDP discovery reader
panic, and any host on the link could send one.

## After

Each of these failures is a WARN line that says what failed, why it matters
and the command that fixes it, under its own doctor code, and is raised on the
report bus, so `ze show warnings` or `ze show errors` and the recent-events
check of `ze doctor` list it. This works whether the protocol runs inside the
daemon or as a forked plugin. The new codes are `doctor-bfd-auth-failed`,
`doctor-bfd-auth-seq-unsaved`, `doctor-bfd-send-failed`,
`doctor-isis-hello-send-failed`, `doctor-ldp-hello-send-failed`,
`doctor-ldp-hello-discarded`, `doctor-ldp-session-ended` and
`doctor-ospfv3-ipsec-entry-left`. A refused event or BFD release is reported
under `doctor-internal-defect`. A warning clears itself when the condition
ends: the Hello goes out again, a packet verifies, a send lands, or the
interface or session is removed.

A failure that can repeat on every packet or every Hello is counted, and only
the first of each run, or the first after a quiet minute, is logged. New
counters: `ze_bfd_send_failures_total{mode,packet}`,
`ze_ldp_hello_send_errors_total{interface}`,
`ze_ldp_hello_discards_total{interface}` and
`ze_isis_lsp_change_events_refused_total`.

The fix line of an OSPFv3 IPsec entry Ze could not delete is the `ip -6 xfrm`
command that deletes it, and never contains the key.

An LDP session the neighbor ends is now logged at WARN as
`LDP session with <peer> ended`, with its cause. A session Ze stopped itself is
logged at info as `ldp: session closed`. The fatal Notification that ends a
session is logged at info, because the session-end WARN already carries its
status. A Hello whose message length is below 4 is discarded and counted like
every other malformed discovery datagram.

## Corrects

none

## Exposure

The LDP Hello panic was reachable on every interface running LDP discovery,
from any host on that link. Nothing else in this change was exposed.

## Upstream

none

## Asked by

none
