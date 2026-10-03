# One ADD-PATH session announces 10.0.0.0/24 twice, under two Path Identifiers
# (RFC 7911 Section 3). Ze dials this injector, which mirrors Ze's OPEN, so the
# session negotiates ADD-PATH send/receive for ipv4/unicast.
#
# THE ORDER IS THE DISCRIMINATION. The better path arrives FIRST:
#   path id 2, MED 10, then path id 1, MED 50.
# A RIB that elects the two paths together keeps path id 2 when path id 1 lands,
# so its Loc-RIB never carries the MED 50 path. A RIB that elects per path
# identifier sees path id 1 as the only candidate of its own election and
# installs it as best, which pmacct then reads in the Loc-RIB stream.
#
# The two paths also differ in their second AS, and that is how pmacct tells
# them apart on the Loc-RIB row: Ze's Loc-RIB Route Monitoring carries ORIGIN,
# AS_PATH and the next hop but not MED (journal class
# constant-reported-as-measured-state). Both AS_PATHs are two ASNs long, so
# length ties and MED decides.
#
# Both UPDATEs, eBGP from AS 65004:
#   ORIGIN:     40010100                    IGP
#   AS_PATH:    40020A02020000FDEC0000FDF2  AS_SEQUENCE[65004 65010] (path id 2)
#               40020A02020000FDEC0000FE1A  AS_SEQUENCE[65004 65050] (path id 1)
#   NEXT_HOP:   400304AC1E0009              172.30.0.9
#   MED:        8004040000000A              10  (path id 2)
#               80040400000032              50  (path id 1)
#   NLRI:       0000000N180A0000            path id N, then 10.0.0.0/24
# attr len = 4+13+7+7 = 31 = 0x1F; total = 19+2+2+31+8 = 62 = 0x3E.
# MED is compared because both paths share the neighbor AS (RFC 4271 Section
# 9.1.2.2 step c).
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=180s
option=linger:value=true

action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003E020000001F4001010040020A02020000FDEC0000FDF2400304AC1E00098004040000000A00000002180A0000
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003E020000001F4001010040020A02020000FDEC0000FE1A400304AC1E00098004040000003200000001180A0000
