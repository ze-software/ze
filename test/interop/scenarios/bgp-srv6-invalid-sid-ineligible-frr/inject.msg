# Raw BGP injector for the RFC 9252 Section 7 SRv6 ineligibility scenario.
# It learns first: ze relays FRR's IPv6 VPN route (RD 65001:1,
# 2001:db8:9252:1::/64) to this eBGP session, and nothing is sent until that
# UPDATE has arrived, so FRR's valid path is in ze's RIB before the competing
# path. The expectation matches the RD and prefix only: ze's VPN route key
# strips the label stack (nlrisplit keyVPN), so the label FRR chose is not
# needed to compete, and this path carries its own (256).
#
# The competing path: the same RD and prefix, next hop ::ffff:172.30.0.9, and
# a Prefix-SID holding one SRv6 L3 Service TLV, SID 2001:db8:1::, End.DT6,
# whose SID Structure is LBL 32, LNL 152, FL 16, AL 0, length 16, offset 48.
# The sum 200 exceeds 128 (RFC 9252 Section 3.2.1), so the SID is invalid, and
# "The path having any such Prefix-SID attribute without any valid SRv6 SID
# information MUST be considered ineligible during the selection of the best
# path for the corresponding prefix." (RFC 9252 Section 7). The framing is
# sound, so the RFC 7606 walk keeps the path. ze's import policy gives this
# peer LOCAL_PREF 200, so without the eligibility gate this path would win.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Hold the session open after the last directive, so ze keeps the path while
# the checker reads the best table.
option=linger:value=true

expect=bgp:conn=1:seq=1:contains=0000FDE90000000120010DB892520001

action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF008002000000694001010040020602010000FDEC800E3100028018000000000000000000000000000000000000FFFFAC1E000900980010010000FDE90000000120010DB892520001C028250500220001001E0020010DB800010000000000000000000000001200010006209810001030
