# An option set for one caller changes what an unrelated call answers

A socket, a file, a process or a kernel object carries one option table, and
every caller shares it. A feature installs an option for its own read path
and, on the same object, the kernel now answers a DIFFERENT call with the
consequence of that option: an error is reported to a later, unrelated write
as that write's own failure, or a read that used to block now returns. The
feature's tests are green, because they exercise the path the option was set
for. The caller that changed is the one nobody re-tested.

The tell is an error that names a datagram, a request or a file the failing
call never touched. The fix is at the object: every caller that shares it
learns what the option hands them, and the page that describes the object
says which option is on and what it does to each call.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-16 | spec-ike-padded-path-probe | `UDPTransport.write` (`internal/component/ike/transport/udp.go`) on the two IKE sockets, once `probe.EnableErrorQueue` installs `IP_RECVERR` at creation for the probe's error-queue drain | With `IP_RECVERR` set, the kernel hands an ICMP error about an EARLIER datagram to the NEXT send on the socket as that send's failure (`sock_alloc_send_pskb` returns the pending `sk_err` before it allocates) and never sends that datagram; before the option an unconnected UDP socket dropped the error, so `Send` had never failed this way. Any SA's message could vanish once a router had refused another SA's DF probe. Found by phase 2's integration test on a clamped router | fixed: `write` drains the error queue on a failed write, delivers what it found on `Refusals()`, and retries the write once; a drain that finds nothing keeps the error as this write's own, and a `LOCAL` entry is the cache refusing this write, not retried. `TestSendSurvivesAnErrorQueuedForAnEarlierDatagram`; the page is `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, "The two IKE sockets" |
