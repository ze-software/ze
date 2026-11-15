# Send only after FRR's own readiness prefix reaches the source. This joins
# the ordinary receiver's establishment with a working forwarding path.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
# The raw peer's completion enters holdSession. The lab's bounded receiver
# checks and container teardown own its lifetime, rather than periodic traffic.
option=linger:value=true
expect=bgp:conn=1:seq=1:contains=180A1400
# ORIGIN IGP, AS_PATH 65004, NEXT_HOP 172.30.0.9, NLRI 10.10.0.0/24.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
