# Beat 1: the lab and section overview, before one SSH candidate.
Type "ze-demo card topic-ipsec lab"
Enter
Wait+Screen /====/
Sleep 5s
Type "ze-demo card topic-ipsec overview"
Enter
Wait+Screen /====/
Sleep 6s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-ipsec ike
Type "load file absolute merge /src/demos/terminal/topics/ipsec/1-ike.conf"
Enter
Wait+Screen /merged from \S*1-ike\.conf/
Card topic-ipsec esp
Type "load file absolute merge /src/demos/terminal/topics/ipsec/2-esp.conf"
Enter
Wait+Screen /merged from \S*2-esp\.conf/
Card topic-ipsec peer
Type "load file absolute merge /src/demos/terminal/topics/ipsec/3-peer.conf"
Enter
Wait+Screen /merged from \S*3-peer\.conf/
Card topic-ipsec xfrm
Type "load file absolute merge /src/demos/terminal/topics/ipsec/4-xfrm.conf"
Enter
Wait+Screen /merged from \S*4-xfrm\.conf/
Type "show | compare"
Enter
Wait+Screen /vpn-xfrm/
Sleep 3s
Type "commit now"
Enter
Wait+Screen /Session committed/
Type "exit"
Enter
Wait+Screen /ze>/
Type "exit"
Enter
Wait+Screen /\$ /
