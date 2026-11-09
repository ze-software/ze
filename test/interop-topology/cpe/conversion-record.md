# Conversion record: vyos.conf to ze-cpe.conf

The model's record of the home-CPE lab conversion, in the format of
`docs/guide/migrating-from-vyos.md`, "The conversion record": one row for every
line of `vyos.conf`, in input order. The `conversion` scenario
(`internal/le/interoplab/topology/home_lab_record.go`) refuses a source line
with no row, a row whose text differs from its line, and a row with no Ze
form or dropped reason. Every value is invented.

| # | VyOS | Ze | Class | Note |
|---|------|----|-------|------|
| 1 | `set interfaces ethernet eth1 description 'WAN'` | `interface ethernet eth1 description WAN` | equivalent | |
| 2 | `set interfaces ethernet eth2 description 'LAN'` | `interface ethernet eth2 description LAN` | equivalent | |
| 3 | `set interfaces bridge br0 address '192.168.50.1/24'` | `interface bridge br0 unit 0 ipv4 address 192.168.50.1/24` | equivalent | |
| 4 | `set interfaces bridge br0 address '10.99.0.1/24'` | `interface bridge br0 unit 0 ipv4 address 10.99.0.1/24` | equivalent | two subnets on one bridge |
| 5 | `set interfaces bridge br0 member interface eth2` | `interface bridge br0 member eth2` | equivalent | |
| 6 | `set interfaces pppoe pppoe0 authentication password '<<SECRET:pppoe-pppoe0-password>>'` | `interface pppoe-client pppoe0 authentication password "<<SECRET:pppoe-pppoe0-password>>"` | equivalent | placeholder kept |
| 7 | `set interfaces pppoe pppoe0 authentication username 'cpe@lab.example.net'` | `interface pppoe-client pppoe0 authentication username cpe@lab.example.net` | equivalent | |
| 8 | `set interfaces pppoe pppoe0 ip adjust-mss '1400'` | `firewall table wan-mss family ip chain mss term pppoe0-mss then tcp-mss-set 1400` | equivalent | postrouting chain, priority -300 |
| 9 | `set interfaces pppoe pppoe0 mtu '1492'` | `interface pppoe-client pppoe0 mtu 1492` | equivalent | |
| 10 | `set interfaces pppoe pppoe0 no-default-route` | `interface pppoe-client pppoe0 no-default-route` | equivalent | |
| 11 | `set interfaces pppoe pppoe0 source-interface 'eth1'` | `interface pppoe-client pppoe0 source-interface eth1` | equivalent | |
| 12 | `set interfaces vti vti0 address 'fd00:db8:1::2/112'` | `interface xfrm vti0 if-id 10 unit 0 ipv6 address fd00:db8:1::2/112` | equivalent | an xfrm interface with its own if-id |
| 13 | `set interfaces wireguard wg0 address '10.88.0.1/24'` | `interface wireguard wg0 unit 0 ipv4 address 10.88.0.1/24` | equivalent | |
| 14 | `set interfaces wireguard wg0 peer laptop allowed-ips '10.88.0.2/32'` | `interface wireguard wg0 peer laptop allowed-ips 10.88.0.2/32` | equivalent | |
| 15 | `set interfaces wireguard wg0 peer laptop preshared-key '<<SECRET:wg-wg0-laptop-psk>>'` | `interface wireguard wg0 peer laptop preshared-key "<<SECRET:wg-wg0-laptop-psk>>"` | equivalent | placeholder kept |
| 16 | `set interfaces wireguard wg0 peer laptop public-key '<<LAB:wg-wg0-laptop-public-key>>'` | `interface wireguard wg0 peer laptop public-key "<<LAB:wg-wg0-laptop-public-key>>"` | equivalent | lab-issued: derived from the laptop's lab key |
| 17 | `set interfaces wireguard wg0 port '51820'` | `interface wireguard wg0 listen-port 51820` | equivalent | |
| 18 | `set interfaces wireguard wg0 private-key '<<SECRET:wg-wg0-private-key>>'` | `interface wireguard wg0 private-key "<<SECRET:wg-wg0-private-key>>"` | equivalent | placeholder kept |
| 19 | `set protocols static route 0.0.0.0/0 interface pppoe0 distance '10'` | `static table default route 0.0.0.0/0 distance 10 next interface pppoe0` | equivalent | |
| 20 | `set firewall global-options all-ping 'enable'` | `firewall global-options all-ping enable` | equivalent | |
| 21 | `set firewall global-options syn-cookies 'enable'` | `firewall global-options syn-cookies enable` | equivalent | |
| 22 | `set firewall group network-group LAN network '192.168.50.0/24'` | `firewall table v4-input chain input term rule-20-a from source-address 192.168.50.0/24`; `firewall table nat-source chain postrouting term rule-100-a from source-address 192.168.50.0/24` | conditional | one term per prefix, not a set |
| 23 | `set firewall group network-group LAN network '10.99.0.0/24'` | `firewall table v4-input chain input term rule-20-b from source-address 10.99.0.0/24`; `firewall table nat-source chain postrouting term rule-100-b from source-address 10.99.0.0/24` | conditional | one term per prefix, not a set |
| 24 | `set firewall ipv4 input filter default-action 'drop'` | `firewall table v4-input chain input policy drop`; `term default-action then drop` | equivalent | |
| 25 | `set firewall ipv4 input filter default-log` | `firewall table v4-input chain input term default-action then counter log prefix "[ipv4-input-default-D]"` | equivalent | |
| 26 | `set firewall ipv4 input filter rule 10 action 'accept'` | `firewall table v4-input chain input term rule-10 then accept` | equivalent | |
| 27 | `set firewall ipv4 input filter rule 10 state 'established'` | `firewall table v4-input chain input term rule-10 from connection-state established,related` | equivalent | |
| 28 | `set firewall ipv4 input filter rule 10 state 'related'` | the same match as line 27 | equivalent | |
| 29 | `set firewall ipv4 input filter rule 20 action 'accept'` | `term rule-20-a then accept`; `term rule-20-b then accept` | equivalent | |
| 30 | `set firewall ipv4 input filter rule 20 destination port '22'` | `term rule-20-a from destination-port 22`; `term rule-20-b from destination-port 22` | equivalent | |
| 31 | `set firewall ipv4 input filter rule 20 protocol 'tcp'` | `term rule-20-a from protocol tcp`; `term rule-20-b from protocol tcp` | equivalent | |
| 32 | `set firewall ipv4 input filter rule 20 source group network-group 'LAN'` | the source matches of lines 22 and 23 | conditional | |
| 33 | `set firewall ipv4 input filter rule 30 action 'accept'` | `firewall table v4-input chain input term rule-30 then accept` | equivalent | |
| 34 | `set firewall ipv4 input filter rule 30 destination port '51820'` | `term rule-30 from destination-port 51820` | equivalent | |
| 35 | `set firewall ipv4 input filter rule 30 protocol 'udp'` | `term rule-30 from protocol udp` | equivalent | |
| 36 | `set firewall ipv4 input filter rule 40 action 'accept'` | `firewall table v4-input chain input term rule-40 then accept` | equivalent | |
| 37 | `set firewall ipv4 input filter rule 40 protocol 'icmp'` | `term rule-40 from protocol icmp` | equivalent | |
| 38 | `set firewall ipv4 input filter rule 50 action 'accept'` | `firewall table v4-input chain input term rule-50 then accept` | equivalent | |
| 39 | `set firewall ipv4 input filter rule 50 destination port '67'` | `term rule-50 from destination-port 67` | equivalent | |
| 40 | `set firewall ipv4 input filter rule 50 inbound-interface name 'br0'` | `term rule-50 from input-interface br0` | equivalent | |
| 41 | `set firewall ipv4 input filter rule 50 protocol 'udp'` | `term rule-50 from protocol udp` | equivalent | |
| 42 | `set firewall ipv6 input filter default-action 'drop'` | `firewall table v6-input chain input policy drop`; `term default-action then drop` | equivalent | |
| 43 | `set firewall ipv6 input filter rule 10 action 'accept'` | `firewall table v6-input chain input term rule-10 then accept` | equivalent | |
| 44 | `set firewall ipv6 input filter rule 10 state 'established'` | `firewall table v6-input chain input term rule-10 from connection-state established,related` | equivalent | |
| 45 | `set firewall ipv6 input filter rule 10 state 'related'` | the same match as line 44 | equivalent | |
| 46 | `set firewall ipv6 input filter rule 20 action 'accept'` | `firewall table v6-input chain input term rule-20 then accept` | equivalent | |
| 47 | `set firewall ipv6 input filter rule 20 protocol 'ipv6-icmp'` | `term rule-20 from protocol icmpv6` | equivalent | `icmpv6` for `ipv6-icmp` |
| 48 | `set nat source rule 100 outbound-interface name 'pppoe0'` | `firewall table nat-source chain postrouting term rule-100-a from output-interface pppoe0`; `term rule-100-b from output-interface pppoe0` | equivalent | |
| 49 | `set nat source rule 100 source group network-group 'LAN'` | the source matches of lines 22 and 23 | conditional | |
| 50 | `set nat source rule 100 translation address 'masquerade'` | `term rule-100-a then masquerade`; `term rule-100-b then masquerade` | equivalent | |
| 51 | `set nat source rule 110 outbound-interface name 'pppoe0'` | `firewall table nat-source chain postrouting term rule-110 from output-interface pppoe0` | equivalent | |
| 52 | `set nat source rule 110 source address '10.88.0.0/24'` | `term rule-110 from source-address 10.88.0.0/24` | equivalent | |
| 53 | `set nat source rule 110 translation address 'masquerade'` | `term rule-110 then masquerade` | equivalent | |
| 54 | `set service dhcp-server listen-interface 'br0'` | `service dhcp-server enabled true`; `listen-interface br0` | equivalent | |
| 55 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 lease '86400'` | `service dhcp-server shared-network LAN subnet 192.168.50.0/24 lease-time 86400` | equivalent | |
| 56 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 option default-router '192.168.50.1'` | `subnet 192.168.50.0/24 default-router 192.168.50.1` | equivalent | |
| 57 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 option name-server '192.0.2.53'` | `subnet 192.168.50.0/24 dns-server 192.0.2.53` | equivalent | |
| 58 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 range 0 start '192.168.50.100'` | `subnet 192.168.50.0/24 range 0 start 192.168.50.100` | equivalent | |
| 59 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 range 0 stop '192.168.50.199'` | `subnet 192.168.50.0/24 range 0 stop 192.168.50.199` | equivalent | |
| 60 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 static-mapping res-a ip-address '192.168.50.30'` | `subnet 192.168.50.0/24 static-mapping res-a ip-address 192.168.50.30` | equivalent | |
| 61 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 static-mapping res-a mac '02:00:5e:50:00:30'` | `subnet 192.168.50.0/24 static-mapping res-a mac-address 02:00:5e:50:00:30` | equivalent | |
| 62 | `set service dhcp-server shared-network-name LAN subnet 192.168.50.0/24 subnet-id '1'` | dropped: Ze keys a subnet by its prefix | not read | |
| 63 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 lease '86400'` | `service dhcp-server shared-network LAN subnet 10.99.0.0/24 lease-time 86400`; `subnet 10.99.0.0/24 dns-source source configured` | equivalent | the subnet has no `option name-server`, so VyOS's kea mapping sends no option 6 (`kea_parse_options` skips an absent `name_server`); the written list keeps Ze from handing the subnet the resolvers it learned (guide, "a subnet with no `option name-server`") |
| 64 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 option default-router '10.99.0.1'` | `subnet 10.99.0.0/24 default-router 10.99.0.1` | equivalent | |
| 65 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 range 0 start '10.99.0.100'` | `subnet 10.99.0.0/24 range 0 start 10.99.0.100` | equivalent | |
| 66 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 range 0 stop '10.99.0.199'` | `subnet 10.99.0.0/24 range 0 stop 10.99.0.199` | equivalent | |
| 67 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 static-mapping res-b ip-address '10.99.0.40'` | `subnet 10.99.0.0/24 static-mapping res-b ip-address 10.99.0.40` | equivalent | |
| 68 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 static-mapping res-b mac '02:00:5e:50:00:40'` | `subnet 10.99.0.0/24 static-mapping res-b mac-address 02:00:5e:50:00:40` | equivalent | |
| 69 | `set service dhcp-server shared-network-name LAN subnet 10.99.0.0/24 subnet-id '2'` | dropped: Ze keys a subnet by its prefix | not read | |
| 70 | `set pki ca LAB-CA certificate '<<LAB:pki-LAB-CA-certificate>>'` | `pki ca LAB-CA certificate "<<LAB:pki-LAB-CA-certificate>>"` | equivalent | lab-issued |
| 71 | `set pki certificate CPE certificate '<<LAB:pki-CPE-certificate>>'` | `pki certificate CPE certificate "<<LAB:pki-CPE-certificate>>"` | equivalent | lab-issued |
| 72 | `set pki certificate CPE private key '<<SECRET:pki-CPE-key>>'` | `pki certificate CPE private key "<<SECRET:pki-CPE-key>>"` | equivalent | placeholder kept; the lab issues it with the certificate |
| 73 | `set vpn ipsec esp-group ESP-MGMT pfs 'disable'` | `vpn ipsec esp-group ESP-MGMT pfs disable` | equivalent | |
| 74 | `set vpn ipsec esp-group ESP-MGMT proposal 1 encryption 'aes128gcm128'` | `vpn ipsec esp-group ESP-MGMT proposal 1 encryption aes128gcm` | equivalent | `aes128gcm` for `aes128gcm128` |
| 75 | `set vpn ipsec ike-group IKE-MGMT key-exchange 'ikev2'` | `vpn ipsec ike-group IKE-MGMT key-exchange ikev2` | equivalent | |
| 76 | `set vpn ipsec ike-group IKE-MGMT proposal 1 dh-group '19'` | `vpn ipsec ike-group IKE-MGMT proposal 1 dh-group 19` | equivalent | |
| 77 | `set vpn ipsec ike-group IKE-MGMT proposal 1 encryption 'aes128gcm128'` | `vpn ipsec ike-group IKE-MGMT proposal 1 encryption aes128gcm` | equivalent | `aes128gcm` for `aes128gcm128` |
| 78 | `set vpn ipsec ike-group IKE-MGMT proposal 1 hash 'sha256'` | `vpn ipsec ike-group IKE-MGMT proposal 1 hash sha256` | equivalent | |
| 79 | `set vpn ipsec interface 'pppoe0'` | `vpn ipsec interface pppoe0`; `vpn ipsec unmatched bypass` | equivalent | VyOS's strongSwan installs no SPD catch-all, so the router's own DNS, NTP, ICMP and PPP-side traffic crosses in the clear; Ze's catch-all defaults to discard, so `bypass` keeps that |
| 80 | `set vpn ipsec site-to-site peer mgmt authentication local-id 'cpe.lab.example.net'` | `vpn ipsec site-to-site peer mgmt authentication local-id cpe.lab.example.net` | equivalent | |
| 81 | `set vpn ipsec site-to-site peer mgmt authentication mode 'x509'` | `peer mgmt authentication mode x509` | equivalent | |
| 82 | `set vpn ipsec site-to-site peer mgmt authentication remote-id 'vpn.example.net'` | `peer mgmt authentication remote-id vpn.example.net` | equivalent | |
| 83 | `set vpn ipsec site-to-site peer mgmt authentication x509 ca-certificate 'LAB-CA'` | `peer mgmt authentication ca-certificate LAB-CA` | equivalent | |
| 84 | `set vpn ipsec site-to-site peer mgmt authentication x509 certificate 'CPE'` | `peer mgmt authentication certificate CPE` | equivalent | |
| 85 | `set vpn ipsec site-to-site peer mgmt connection-type 'initiate'` | `peer mgmt connection-type initiate` | equivalent | |
| 86 | `set vpn ipsec site-to-site peer mgmt ike-group 'IKE-MGMT'` | `peer mgmt ike-group IKE-MGMT` | equivalent | |
| 87 | `set vpn ipsec site-to-site peer mgmt local-address '203.0.113.1'` | none | conditional | filled from `vpn ipsec interface pppoe0` (child 4) |
| 88 | `set vpn ipsec site-to-site peer mgmt remote-address 'vpn.example.net'` | `peer mgmt remote-address vpn.example.net` | equivalent | |
| 89 | `set vpn ipsec site-to-site peer mgmt vti bind 'vti0'` | `peer mgmt vti bind vti0`; `traffic-selector 1`, `traffic-selector 2` | not read | both families on the VTI |
| 90 | `set vpn ipsec site-to-site peer mgmt vti esp-group 'ESP-MGMT'` | `peer mgmt esp-group ESP-MGMT` | equivalent | |
| 91 | `set service ssh listen-address '::'` | `environment ssh enabled true`; `server main ip ::` | equivalent | SSH on `::` only |
| 92 | `set service ssh port '22'` | `environment ssh server main port 22` | equivalent | |
| 93 | `set system conntrack modules ftp` | `system conntrack module ftp` | equivalent | |
| 94 | `set system conntrack modules nfs` | dropped: no kernel module provides it, and Ze refuses the name | behavior change | |
| 95 | `set system domain-name 'home.example.net'` | `system domain home.example.net` | equivalent | |
| 96 | `set system host-name 'cpe'` | `system host cpe` | equivalent | |
| 97 | `set system login user admin authentication encrypted-password '<<SECRET:user-admin-password>>'` | `system authentication user admin plaintext-password "<<SECRET:user-admin-password>>"` | behavior change | the owner types the password; Ze stores bcrypt |
| 98 | `set system name-server '192.0.2.53'` | `system name-server 192.0.2.53` | equivalent | |
| 99 | `set system update-check auto-check` | dropped: on gokrazy, gokrazy manages the image updates | - | |
