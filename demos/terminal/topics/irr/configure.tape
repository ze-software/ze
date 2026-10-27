Type "ze-demo card topic-irr lab"
Enter
Wait+Screen /====/
Sleep 6s
Type "ze cli -c 'show bgp irr | yaml | no-more'"
Enter
Wait+Screen /entries: \[\]/
Sleep 3s
Type "ze-demo card topic-irr overview"
Enter
Wait+Screen /====/
Sleep 6s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-irr server
Type "load file absolute merge /src/demos/terminal/topics/irr/1-server.conf"
Enter
Wait+Screen /merged from \S*1-server\.conf/
Card topic-irr as-set
Type "load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf"
Enter
Wait+Screen /merged from \S*2-as-set\.conf/
Card topic-irr import
Type "load file absolute merge /src/demos/terminal/topics/irr/3-import.conf"
Enter
Wait+Screen /merged from \S*3-import\.conf/
Type "show | compare"
Enter
Wait+Screen /AS-TEST/
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
Type "ze-demo run showcase-lab control customer reannounce"
Enter
Wait+Screen /Customer routes announced/
Type "ze-demo run irr-filter await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
