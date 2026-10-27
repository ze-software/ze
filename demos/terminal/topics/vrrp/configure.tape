# Shared by the standalone topic and the continuous showcase.
Type "ze-demo card topic-vrrp overview"
Enter
Sleep 6s
Hide
Type "ze-demo run vrrp-tracking await absent"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze cli -c 'show vrrp | no-more'"
Enter
Sleep 4s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-vrrp group
Type "load file absolute merge /src/demos/terminal/topics/vrrp/1-group.conf"
Enter
Wait+Screen /merged from \S*1-group\.conf/
Sleep 3s
Card topic-vrrp tracking
Type "load file absolute merge /src/demos/terminal/topics/vrrp/2-tracking.conf"
Enter
Wait+Screen /merged from \S*2-tracking\.conf/
Sleep 3s
Type "show | compare"
Enter
Sleep 6s
Type "commit now"
Enter
Wait+Screen /Session committed/
Sleep 3s
Type "exit"
Enter
Wait+Screen /operational\]/
Type "exit"
Enter
Wait+Screen /\$ /
