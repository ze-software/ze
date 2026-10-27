Hide
Type "ze-demo run traffic-anomaly await configured"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo card topic-traffic proof"
Enter
Sleep 6s
Type "ip netns exec traffic-peer ping -c 3 -W 2 10.77.0.1"
Enter
Sleep 4s
Type "ip netns exec traffic-peer curl --fail --silent --show-error --max-time 5 --output /dev/null --write-out 'HTTP %{http_code}\\n' http://10.77.0.1:8080/payload.txt"
Enter
Sleep 3s
Hide
Type "ze-demo run traffic-anomaly await accounted"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show traffic usage name traffic0 | no-more'"
Enter
Sleep 4s
Type "ze-demo card topic-traffic failure"
Enter
Sleep 6s
Type "ze-demo run showcase-lab control traffic burst"
Enter
Hide
Type "ze-demo run traffic-anomaly await increasing"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show traffic usage name traffic0 | no-more'"
Enter
Sleep 4s
Hide
Type "ze-demo run traffic-anomaly await increasing"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show traffic usage name traffic0 | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show metrics name ze_traffic_usage_ingress_bytes_total label interface traffic0 label src_ip 10.77.0.2 | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show metrics name ze_traffic_usage_ingress_port_bytes_total label interface traffic0 label dst_port 8080 label protocol tcp | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show metrics name ze_traffic_usage_ingress_port_bytes_total label interface traffic0 label dst_port 0 label protocol icmp | no-more'"
Enter
Sleep 4s
Type "ze-demo card topic-traffic fix"
Enter
Sleep 6s
Type "ze-demo run showcase-lab control traffic stop"
Enter
Hide
Type "ze-demo run traffic-anomaly await settled"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show traffic usage name traffic0 | no-more'"
Enter
Sleep 4s
Sleep 2s
Type "ze cli -c 'show traffic usage name traffic0 | no-more'"
Enter
Sleep 4s
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze-demo card topic-traffic recap"
Enter
Sleep 6s
