---
spec: rfc-verdict-fix-services
topic: RFC Compliance
found-by: rfc-check
---

## Before

When a TFTP client sent the same ACK twice while a block was still
outstanding, Ze's TFTP server answered the duplicate by sending the DATA block
again, the Sorcerer's Apprentice problem that RFC 1350 Section 5 describes. A
tunnel interface configured with remote `0.0.0.0` or `::` was accepted, and
the kernel tunnel it installed decapsulated packets from any source address.

<!-- source: internal/plugins/tftpserver/handler.go -- sendAndWaitACK -->
<!-- source: internal/component/iface/config.go -- parseTunnelEntry -->

## After

The TFTP server ignores a duplicate ACK and resends a block only when its ACK
timeout expires. A test against curl, with a relay that duplicates curl's
ACK, shows Ze silent until the timeout and the download complete and intact.
A tunnel whose remote is `0.0.0.0` or `::` is now refused by
`ze config validate` and by a commit, for every tunnel kind and with either
backend, and the message names the command that sets a real remote. RFC 4213
Section 3.6 requires the decapsulator to verify the tunnel source, and an
unspecified remote verifies nothing.

<!-- source: internal/plugins/tftpserver/tftp_curl_docker_test.go -- TestTFTPCurlDuplicateACKInterop -->
<!-- source: internal/component/iface/tunnel.go -- CheckRemote -->

## Corrects

none

## Exposure

The TFTP fix concerns the PXE TFTP server, and only clients that send
duplicate ACKs, or networks that duplicate packets, saw the extra DATA. The
tunnel fix concerns any tunnel whose remote was written as an unspecified
address, on either backend; a tunnel with a specific remote address was never
exposed.

## Upstream

none

## Asked by

none
