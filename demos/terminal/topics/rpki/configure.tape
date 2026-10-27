Type "ze-demo card topic-rpki lab"
Enter
Wait+Screen /====/
Sleep 6s
Type "ze cli -c 'show bgp rpki status | yaml | no-more'"
Enter
Wait+Screen /running: false/
Sleep 3s
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /11\.43\.0\.0\/24/
Sleep 3s
Hide
Type "ze cli -c 'request log level bgp.rpki info'"
Enter
Wait+Screen /\$ /
Type "clear"
Enter
Show
Type "ze-demo card topic-rpki overview"
Enter
Wait+Screen /====/
Sleep 6s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-rpki cache
Type "load file absolute merge /src/demos/terminal/topics/rpki/1-cache.conf"
Enter
Wait+Screen /merged from \S*1-cache\.conf/
Card topic-rpki policy
Type "load file absolute merge /src/demos/terminal/topics/rpki/2-policy.conf"
Enter
Wait+Screen /merged from \S*2-policy\.conf/
Card topic-rpki attach
Type "load file absolute merge /src/demos/terminal/topics/rpki/3-attach.conf"
Enter
Wait+Screen /merged from \S*3-attach\.conf/
Type "show | compare"
Enter
Wait+Screen /bgp-rpki/
Sleep 3s
Type "commit now"
Enter
Wait+Screen /Session committed/
Type "exit"
Enter
Wait+Screen /operational\]/
Type "exit"
Enter
Wait+Screen /\$ /
Hide
Type "ze-demo run rpki await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
