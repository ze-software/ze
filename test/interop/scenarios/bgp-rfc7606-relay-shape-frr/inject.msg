# Raw BGP injector for the RFC 7606 Section 5.1 relay scenario.
#
# Ze is a route server. Its replayed 10.0.0.0/24 proves relay liveness.
# The checker observes 198.51.100.0/24 installed at FRR before releasing
# the mixed UPDATE with the source-only announced route 192.0.2.0/24.
# That UPDATE withdraws the installed seed and announces 203.0.113.0/24.
# A later source fence 198.51.101.0/24 closes the measured FIFO interval.
#
# FRR 10.7.1 discards later duplicate ordinary attributes, so installation
# does not prove NEXT_HOP uniqueness. The byte-transparent FRR-facing relay
# independently checks the actual split frames and their attributes.
# ASN 65004 and NEXT_HOP 172.30.0.9 are unchanged on this route-server rail.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Keep this exact source session alive for the observer's entire capture.
option=linger:value=true

# Preannounce both replay control and withdrawal seed, then End-of-RIB.
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E000918C63364
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# The existing peer matches only the announced-NLRI section. The checker
# sends this route only after FRR has installed the seed above.
expect=bgp:conn=1:seq=4:announced=18C00002

# ONE mixed UPDATE, then a distinct source-originated completion fence.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003302000418C6336400144001010040020602010000FDEC400304AC1E000918CB0071
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E000918C63365

# FRR's outbound-only NO-RETURN prefix list prevents reflected seed routes
# from racing the explicit release expectation. It does not filter what FRR
# receives. Linger holds the source until timeout or lab teardown.
