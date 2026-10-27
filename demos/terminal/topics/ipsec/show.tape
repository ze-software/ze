# Beat 3: negotiated state, kernel state and real protected packets.
Hide
Type "ze-demo run ipsec await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo card topic-ipsec proof"
Enter
Wait+Screen /====/
Sleep 4s
Type "ze cli -c 'show vpn ipsec status | no-more'"
Enter
Wait+Screen /established-sas/
Type "ze cli -c 'show vpn ipsec sa | no-more'"
Enter
Wait+Screen /inbound-spi/
Sleep 3s
Type "ze cli -c 'show vpn ipsec dataplane sa | no-more'"
Enter
Wait+Screen /replay-window/
Sleep 3s
Type "ze-demo run showcase-lab control vpn ping"
Enter
Wait+Screen /4 received/
Type "ze cli -c 'show vpn ipsec sa | no-more'"
Enter
Wait+Screen /packets-in/
Sleep 3s
# Beat 4: only ESP changes; the far end offers AES-256, not AES-128.
Type "ze-demo card topic-ipsec failure"
Enter
Wait+Screen /====/
Sleep 4s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-ipsec mismatch
Type "load file absolute merge /src/demos/terminal/topics/ipsec/5-mismatch.conf"
Enter
Wait+Screen /merged from \S*5-mismatch\.conf/
Type "show | compare"
Enter
Wait+Screen /aes128/
Type "commit now"
Enter
Wait+Screen /Session committed/
Type "exit"
Enter
Wait+Screen /ze>/
Type "exit"
Enter
Wait+Screen /\$ /
Hide
Type "ze-demo run ipsec await refused"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
# Beat 5: authenticated IKE is not proof of a working ESP tunnel.
Type "ze-demo card topic-ipsec diagnosis"
Enter
Wait+Screen /====/
Sleep 5s
Type "ze cli -c 'show vpn ipsec peer name responder | no-more'"
Enter
Wait+Screen /established/
Sleep 3s
Type "ze cli -c 'show log recent level warn | no-more'"
Enter
Wait+Screen /notify: NO_PROPOSAL_CHOSEN/
Wait+Screen /peer: responder/
Sleep 4s
Type "ze cli -c 'show vpn ipsec dataplane sa | no-more'"
Enter
Wait+Screen /sas: \[\]/
Type "ze cli -c 'show vpn ipsec dataplane drift | no-more'"
Enter
Wait+Screen /drift: \[\]/
Sleep 3s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-ipsec restore
Type "load file absolute merge /src/demos/terminal/topics/ipsec/2-esp.conf"
Enter
Wait+Screen /merged from \S*2-esp\.conf/
Type "show | compare"
Enter
Wait+Screen /aes256/
Type "commit now"
Enter
Wait+Screen /Session committed/
Type "exit"
Enter
Wait+Screen /ze>/
Type "exit"
Enter
Wait+Screen /\$ /
Type "ze cli -c 'clear vpn ipsec sa peer responder | no-more'"
Enter
Wait+Screen /clear-peer/
Hide
Type "ze-demo run ipsec await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo run showcase-lab control vpn ping"
Enter
Wait+Screen /4 received/
Type "ze cli -c 'show vpn ipsec sa | no-more'"
Enter
Wait+Screen /packets-in/
Sleep 3s
Type "ze cli -c 'show vpn ipsec dataplane sa | no-more'"
Enter
Wait+Screen /replay-window/
Type "ze-demo card topic-ipsec recap"
Enter
Wait+Screen /====/
Sleep 4s
