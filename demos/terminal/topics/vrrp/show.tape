Hide
Type "ze-demo run vrrp-tracking await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo card topic-vrrp proof"
Enter
Sleep 6s
Type "ze cli -c 'show vrrp interface name zeptrk0 | no-more'"
Enter
Sleep 4s
Type "ip -br address show"
Enter
Sleep 2s
Type "ip netns exec showcase-vrrp ping -I eth0 -c 3 -W 2 203.0.113.1"
Enter
Sleep 4s
Type "ze-demo card topic-vrrp failure"
Enter
Sleep 6s
Type "ze-demo run showcase-lab control tracked-uplink down"
Enter
Hide
Type "ze-demo run vrrp-tracking await tracked-backup"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show vrrp interface name zeptrk0 | no-more'"
Enter
Sleep 4s
Type "ip link show zetrk1"
Enter
Sleep 2s
Type "ip -n showcase-vrrp -br address show"
Enter
Sleep 2s
Type "ping -I zeptrk0 -c 3 -W 2 203.0.113.1"
Enter
Sleep 4s
Type "ze-demo card topic-vrrp takeover"
Enter
Sleep 6s
Type "ze-demo run showcase-lab control keepalived stop"
Enter
Hide
Type "ze-demo run vrrp-tracking await low-priority-master"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show vrrp interface name zeptrk0 | no-more'"
Enter
Sleep 4s
Type "ip -br address show"
Enter
Sleep 2s
Type "ip -n showcase-vrrp -br address show"
Enter
Sleep 2s
Type "ip netns exec showcase-vrrp ping -I eth0 -c 3 -W 2 203.0.113.1"
Enter
Sleep 4s
Type "ze-demo card topic-vrrp fix"
Enter
Sleep 6s
Type "ze-demo run showcase-lab control keepalived start"
Enter
Hide
Type "ze-demo run vrrp-tracking await peer-restored"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show vrrp interface name zeptrk0 | no-more'"
Enter
Sleep 4s
Type "ip -n showcase-vrrp -br address show"
Enter
Sleep 2s
Type "ze-demo run showcase-lab control tracked-uplink up"
Enter
Hide
Type "ze-demo run vrrp-tracking await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show vrrp interface name zeptrk0 | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show vrrp statistics | no-more'"
Enter
Sleep 4s
Type "ip -br link show"
Enter
Sleep 2s
Type "ip netns exec showcase-vrrp ping -I eth0 -c 3 -W 2 203.0.113.1"
Enter
Sleep 4s
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze-demo card topic-vrrp recap"
Enter
Sleep 6s
