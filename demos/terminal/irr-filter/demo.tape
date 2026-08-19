Output artifacts/irr-filter.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/irr-filter/config ZE_INIT_INPUT=/src/tmp/terminal-demos/state/irr-filter/init.input ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "/src/demos/terminal/irr-filter/run.sh prepare"
Enter
Wait+Screen /IRR filter demo prepared/
Type "ze init < $ZE_INIT_INPUT >/dev/null"
Enter
Type "/src/demos/terminal/irr-filter/run.sh seed"
Enter
Wait+Screen /without IRR filtering/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh irr-filter intro"
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
Wait+Screen /not configured/
Sleep 4s
Type "ze config set ze.conf plugin internal bgp-filter-irr use bgp-filter-irr"
Enter
Wait+Screen /set plugin internal bgp-filter-irr use bgp-filter-irr/
Sleep 3s
Type "ze config set ze.conf bgp policy irr server 127.0.0.1:4343"
Enter
Wait+Screen /set bgp policy irr server 127.0.0.1:4343/
Sleep 3s
Type "ze config set ze.conf bgp policy irr refresh-interval 3600"
Enter
Wait+Screen /set bgp policy irr refresh-interval 3600/
Sleep 3s
Type "ze config set ze.conf bgp peer customer-a session irr as-set AS-TEST"
Enter
Wait+Screen /set bgp peer customer-a session irr as-set AS-TEST/
Sleep 3s
Type "ze config set ze.conf bgp peer customer-a filter import bgp-filter-irr:65001"
Enter
Wait+Screen /set bgp peer customer-a filter import bgp-filter-irr:65001/
Sleep 5s
Type "clear"
Enter
Type "ze config cat ze.conf | grep -E 'bgp-filter-irr|AS-TEST|127.0.0.1:4343'"
Enter
Wait+Screen /AS-TEST/
Sleep 7s

Hide
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh irr-filter configured"
Enter
Show
Sleep 9s

Hide
Type "/src/demos/terminal/irr-filter/run.sh start"
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
Type "/src/demos/terminal/irr-filter/run.sh announce"
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
Type "/src/demos/terminal/irr-filter/run.sh stop"
Enter
Wait+Screen /IRR filter demo stopped/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh irr-filter recap"
Enter
Show
Sleep 9s
