# IRR topic, beat 3: the proof, from the shell after the editor has exited.
# The customer connects only now, so its routes meet the committed filter.
Type "ze cli -c 'show bgp irr | no-more'"
Enter
Wait+Screen /AS-TEST/
Sleep 7s
Type "clear"
Enter
Type "ze cli -c 'show bgp irr prefix customer | no-more'"
Enter
Wait+Screen /2001:db8::\/32/
Sleep 7s

Hide
Type "ze-demo run showcase-lab announce"
Enter
Wait+Screen /Customer routes announced/
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp irr check customer 10.0.0.0/24 | no-more | yaml'"
Enter
Wait+Screen /accepted: true/
Sleep 5s
Type "ze cli -c 'show bgp irr check customer 192.168.0.0/24 | no-more | yaml'"
Enter
Wait+Screen /accepted: false/
Sleep 6s
Type "clear"
Enter
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /10\.0\.0\.0\/24/
Sleep 7s
