Type "ze-demo card topic-rpki proof"
Enter
Wait+Screen /====/
Sleep 5s
Type "ze cli -c 'show bgp rpki status | yaml | no-more'"
Enter
Wait+Screen /sessions-synced: 1/
Sleep 3s
Type "ze cli -c 'show bgp rpki summary | yaml | no-more'"
Enter
Wait+Screen /validation-enabled: true/
Sleep 3s
Type "ze cli -c 'show bgp adj-rib-in | yaml | no-more'"
Enter
Wait+Screen /ineligible: true/
Sleep 5s
Type "ze-demo card topic-rpki origin"
Enter
Wait+Screen /====/
Sleep 5s
Type "ze cli -c 'show bgp rpki roa 10.43.0.0/24 | yaml | no-more'"
Enter
Wait+Screen /asn: 65099/
Sleep 4s
Type "ze-demo card topic-rpki failure"
Enter
Wait+Screen /====/
Sleep 5s
Hide
Type "ze-demo run rpki await outage-boundary"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo run showcase-lab control rtr-cache stop"
Enter
Wait+Screen /\$ /
Hide
Type "ze-demo run rpki await down"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show log recent component bgp.rpki level warn count 3 | yaml | no-more'"
Enter
Wait+Screen /cache did not answer/
Sleep 5s
Type "ze cli -c 'show bgp rpki cache | yaml | no-more'"
Enter
Wait+Screen /retry-interval: 1/
Sleep 3s
Type "ze cli -c 'show bgp rpki status | yaml | no-more'"
Enter
Wait+Screen /vrp-count-ipv4: 171/
Sleep 4s
Type "ze-demo card topic-rpki recovery"
Enter
Wait+Screen /====/
Sleep 5s
Type "ze-demo run showcase-lab control rtr-cache start"
Enter
Wait+Screen /\$ /
Hide
Type "ze-demo run rpki await recovered"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show log recent component bgp.rpki count 4 | yaml | no-more'"
Enter
Wait+Screen /sync complete/
Sleep 4s
Type "ze cli -c 'show bgp rpki status | yaml | no-more'"
Enter
Wait+Screen /sessions-synced: 1/
Type "ze cli -c 'show bgp peer detail transit | yaml | match connections-established | no-more'"
Enter
Wait+Screen /connections-established: 1/
Sleep 4s
