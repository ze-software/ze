# Shared by the standalone topic and the continuous showcase.
Type "ze-demo card topic-traffic overview"
Enter
Sleep 6s
Hide
Type "ze-demo run traffic-anomaly await absent"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze cli -c 'show traffic usage | no-more'"
Enter
Sleep 4s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-traffic accounting
Type "load file absolute merge /src/demos/terminal/topics/traffic/1-accounting.conf"
Enter
Wait+Screen /merged from \S*1-accounting\.conf/
Sleep 3s
Card topic-traffic interface
Type "load file absolute merge /src/demos/terminal/topics/traffic/2-interface.conf"
Enter
Wait+Screen /merged from \S*2-interface\.conf/
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
