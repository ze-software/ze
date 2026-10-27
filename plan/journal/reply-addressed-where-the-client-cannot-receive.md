# A reply is addressed where its client cannot receive it

A server answers a client that has no usable address yet, and picks a
destination the client cannot take delivery at: an IP unicast whose link-layer
half needs an ARP answer the client cannot give, or a unicast to an address
the RFC says the reply must not go to. The reply is built correctly and every
unit test that reads the chosen address agrees with the code; only a real
client on a real link shows that nothing arrives.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-27 | spec-lan-dns-resolver | `responseAddr` (`internal/plugins/dhcpserver/register.go`) | BROADCAST bit clear, giaddr and ciaddr zero: OFFER/ACK unicast to yiaddr with no chaddr neighbor entry, ARP never answered, reply lost (RFC 2131 4.1); a giaddr-zero NAK went to ciaddr | fixed: `replyDestination` installs yiaddr -> chaddr on the receiving interface before the unicast and broadcasts if it cannot; every giaddr-zero NAK is broadcast (`rfc2131_delivery_test.go`) |
