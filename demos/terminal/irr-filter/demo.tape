Output artifacts/irr-filter.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/irr-filter/config ZE_INIT_INPUT=/src/tmp/terminal-demos/state/irr-filter/init.input ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "ze-demo run irr-filter prepare"
Enter
Wait+Screen /IRR filter demo prepared/
Type "ze init < $ZE_INIT_INPUT >/dev/null"
Enter
Type "ze-demo run irr-filter seed"
Enter
Wait+Screen /without IRR filtering/
Type "clear"
Enter
Type "ze-demo card irr-filter intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/irr-filter.png

Hide
Type "clear"
Enter
Show
Type "ze config cat ze.conf | grep -q bgp-filter-irr || echo 'IRR filtering is not configured'"
Enter
Wait+Screen /(?m)^IRR filtering is not configured/
Sleep 4s
# An offline `ze config set` edits the file path it is given, never a stored
# name, so the stored ze.conf goes through stdin: read it, apply each setting,
# store the result. A line ending in a pipe continues on the next prompt.
Type "ze config cat ze.conf |"
Enter
Type "ze config set - plugin internal bgp-filter-irr use bgp-filter-irr |"
Enter
Type "ze config set - bgp policy irr server 127.0.0.1:4343 |"
Enter
Type "ze config set - bgp policy irr refresh-interval 3600 |"
Enter
Type "ze config set - bgp peer customer-a session irr as-set AS-TEST |"
Enter
Type "ze config set - bgp peer customer-a filter import bgp-filter-irr:65001 |"
Enter
Type "ze config import --yes --name ze.conf -"
Enter
Wait+Screen /(?m)^1 file\(s\) imported/
Sleep 8s
Type "clear"
Enter
Type "ze config cat ze.conf | grep -E 'bgp-filter-irr|AS-TEST|127.0.0.1:4343'"
Enter
# An output line of the stored config: indented, and holding the leaf name.
Wait+Screen /(?m)^\s+as-set AS-TEST/
Sleep 7s

Hide
Type "clear"
Enter
Type "ze-demo card irr-filter configured"
Enter
Show
Sleep 9s

Hide
Type "ze-demo run irr-filter start"
Enter
Wait+Screen /IRR filter demo ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp irr | no-more'"
Enter
Wait+Screen /AS-TEST/
Sleep 7s
Type "clear"
Enter
Type "ze cli -c 'show bgp irr prefix customer-a | no-more'"
Enter
Wait+Screen /2001:db8::\/32/
Sleep 7s

Hide
Type "ze-demo run irr-filter announce"
Enter
Wait+Screen /Customer routes announced/
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp irr check customer-a 10.0.0.0/24 | no-more | yaml'"
Enter
Wait+Screen /accepted: true/
Sleep 5s
Type "ze cli -c 'show bgp irr check customer-a 192.168.0.0/24 | no-more | yaml'"
Enter
Wait+Screen /accepted: false/
Sleep 6s
Type "clear"
Enter
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /10.0.0.0\/24/
Sleep 7s

Hide
Type "ze-demo run irr-filter stop"
Enter
Wait+Screen /IRR filter demo stopped/
Type "clear"
Enter
Type "ze-demo card irr-filter recap"
Enter
Show
Sleep 9s
