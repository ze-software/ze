# Raw BGP injector for the RFC 8669 Section 6 / RFC 9252 Section 7 malformed
# Prefix-SID scenario. The five UPDATEs are the frames of
# test/plugin/prefixsid-malformed-receive.ci (cases a, d, c, b1, b2), with the
# AS_PATH rewritten to 65004 and every next hop to 172.30.0.9 (this container):
# lengths are unchanged. No conforming speaker sends these attributes, so the
# raw injector writes the bytes.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Hold the session open after the last directive, so ze does not withdraw the
# relayed routes before FRR is asked about them.
option=linger:value=true

# (a) 10.0.1.0/24 labeled unicast, Label-Index TLV of length 6: malformed
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0043020000002C4001010040020602010000FDEC800E1000010404AC1E00090030003E810A0001C02809010006000000000000

# (d) 10.0.4.0/24 labeled unicast, Originator SRGB TLV only: invalid
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0045020000002E4001010040020602010000FDEC800E1000010404AC1E00090030003E810A0004C0280B0300080000003E800003E8

# (c) 10.0.3.0/24 VPN, Label-Index of length 6 then a valid L3 Service TLV: the control
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF007802000000614001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070007D010000FDE9000000010A0003C0282E0100060000000000000500220001001E0020010DB800010000000000000000000000001300010006201010000000

# (b1) 10.0.2.0/24 VPN, a valid SRv6 L3 Service TLV
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF006F02000000584001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070007D010000FDE9000000010A0002C028250500220001001E0020010DB800010000000000000000000000001300010006201010000000

# (b2) 10.0.2.0/24 VPN again, the L3 Service TLV length 39 over 34: treat-as-withdraw
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF006F02000000584001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070007D010000FDE9000000010A0002C028250500270001001E0020010DB800010000000000000000000000001300010006201010000000
