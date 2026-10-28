#!/bin/bash
set -euo pipefail
cd '/home/thomas/Code/github.com/ze-software/ze/gh-pages'

# ze-commit-script: tmp/commit-408ec569-a-1743da.sh session=408ec569

# Commit a: site: publish WebP screenshots and terminal-event playback
# ze-commit-block: tag=a paths='assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 'index.html' 'index.md' 'features/web-interface/index.html' 'features/web-interface/index.md' 'guides/bfd/index.html' 'guides/bfd/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'guides/ipsec/index.html' 'guides/ipsec/index.md' 'guides/irr-filtering/index.html' 'guides/irr-filtering/index.md' 'guides/ospf/index.html' 'guides/ospf/index.md' 'guides/rpki/index.html' 'guides/rpki/index.md' 'guides/traffic-usage/index.html' 'guides/traffic-usage/index.md' 'guides/vrrp/index.html' 'guides/vrrp/index.md' 'data/search-index.json' 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png'
_ze_index="$PWD"/'tmp/commit-408ec569-a-1743da.index'
_ze_message='tmp/commit-msg-408ec569-a-68f584.txt'
if [ ! -f "$_ze_message" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not run, and HEAD is unchanged by it.' >&2
  echo "Its message file $_ze_message is gone. A block deletes it once its commit lands, so this block already published on an earlier run of this script, or the file was cleaned up." >&2
  echo 'Check whether it landed with: git log --format='"'"'%h %s'"'"' --fixed-strings --grep='"'"'site: publish WebP screenshots and terminal-event playback'"'"'' >&2
  echo "Then prepare whatever is still uncommitted again with ./le commit create, rather than running this script again." >&2
  exit 1
fi
_ze_text=$(git stripspace < "$_ze_message")
if [ -z "$_ze_text" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "Its message file $_ze_message holds no message once blank lines are stripped, and git commit refuses an empty message too." >&2
  echo "Write the message into that file, then run this script again." >&2
  exit 1
fi
_ze_status=0
_ze_gpgsign=$(git config --type=bool --get commit.gpgsign) || _ze_status=$?
if [ "$_ze_status" -gt 1 ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "git config could not read commit.gpgsign as a boolean (its error is above), so whether to sign is unknown; git commit refuses here too." >&2
  echo "Set it with: git config commit.gpgsign true (or false), then run this script again." >&2
  exit 1
fi
_ze_sign=
if [ "$_ze_gpgsign" = true ]; then _ze_sign=1; fi
_ze_parent='a321dc1c304e3e82ae2b8c34eee044e1bc5da74e'
_ze_attempt=1
while :; do
_ze_seen=$_ze_parent
_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')
_ze_moved=$(git --literal-pathspecs rev-list "$_ze_seen..$_ze_parent" -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 'index.html' 'index.md' 'features/web-interface/index.html' 'features/web-interface/index.md' 'guides/bfd/index.html' 'guides/bfd/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'guides/ipsec/index.html' 'guides/ipsec/index.md' 'guides/irr-filtering/index.html' 'guides/irr-filtering/index.md' 'guides/ospf/index.html' 'guides/ospf/index.md' 'guides/rpki/index.html' 'guides/rpki/index.md' 'guides/traffic-usage/index.html' 'guides/traffic-usage/index.md' 'guides/vrrp/index.html' 'guides/vrrp/index.md' 'data/search-index.json' 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png')
_ze_peer=$(printf '%s\n' "$_ze_moved" | grep -v -x -F -f <(printf '%s\n' ${_ze_own:-}) || [ "$?" -eq 1 ])
_ze_names=
if [ -n "$_ze_peer" ]; then
  _ze_names=$(printf '%s\n' "$_ze_peer" | git --literal-pathspecs -c core.quotePath=false diff-tree --stdin -r --no-renames --name-only --no-commit-id -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 'index.html' 'index.md' 'features/web-interface/index.html' 'features/web-interface/index.md' 'guides/bfd/index.html' 'guides/bfd/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'guides/ipsec/index.html' 'guides/ipsec/index.md' 'guides/irr-filtering/index.html' 'guides/irr-filtering/index.md' 'guides/ospf/index.html' 'guides/ospf/index.md' 'guides/rpki/index.html' 'guides/rpki/index.md' 'guides/traffic-usage/index.html' 'guides/traffic-usage/index.md' 'guides/vrrp/index.html' 'guides/vrrp/index.md' 'data/search-index.json' 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png' | sort -u)
fi
if [ -n "$_ze_peer" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "A commit that landed after it was prepared changed paths it carries:" >&2
  printf '%s\n' "$_ze_names" >&2
  echo "Building on that commit would write content prepared before the change over it." >&2
  echo "Read the change with: git log -p $_ze_seen..$_ze_parent -- <path>" >&2
  echo "then fold it into your copy and prepare the commit again with ./le commit create." >&2
  exit 1
fi
rm -f "$_ze_index"
GIT_INDEX_FILE="$_ze_index" git read-tree "$_ze_parent"
GIT_INDEX_FILE="$_ze_index" git update-index --index-info <<'ZE_INDEX_INFO'
100644 38dfe1f36fe4cff87cc0932f457f265b5653d3da 0	assets/demos/bfd-failover.cast
100644 bdef97f8449812114bd8de6422e9a5e10276e9aa 0	assets/demos/bfd-failover.txt
100644 7302016eb5e14b1ea0c366fb209c605d41d424de 0	assets/demos/commit-confirmed.cast
100644 d65f7ccff4d0a8430da8f583899629b1332048ad 0	assets/demos/commit-confirmed.txt
100644 ea97cc698a9e6ae0676dc7a7eedd7d72ceab0207 0	assets/demos/ipsec.cast
100644 632bb8932935e5987a8dde4f15409458cfa8b18f 0	assets/demos/ipsec.txt
100644 bab468c25ac4da910296c1135fc992f9974c2bf9 0	assets/demos/irr-filter.cast
100644 d580a21dabae4e8f278a633dbaca920b3708805c 0	assets/demos/irr-filter.txt
100644 2173e9ba7304ffedd86efaf5acc535452905275e 0	assets/demos/manifest.json
100644 382df12b5f68f7ab5780dd591bab5499681cb63e 0	assets/demos/ospf-adjacency.cast
100644 194cb00ce3caaa465a0ba40566a5ce96ed275d3a 0	assets/demos/ospf-adjacency.txt
100644 e46a93199885651cd87080f28b3b245ab4840a2d 0	assets/demos/rpki.cast
100644 f90e8fc82bba7587bda3e42654019a133f75aa5e 0	assets/demos/rpki.txt
100644 09385c0350a5be5b2cf126d358e4f237434a9b47 0	assets/demos/showcase.cast
100644 0ea3be00dbe3e3fe931d79ab0336866b44a91eec 0	assets/demos/showcase.txt
100644 0d4c0cb9800d559f17f23edf4d68f9c464526d83 0	assets/demos/traffic-anomaly.cast
100644 90920c6f0c343eaab3c1b44ca7684b1c3b9d0440 0	assets/demos/traffic-anomaly.txt
100644 d9df42db49447fb423d445949f3b413cc490be1f 0	assets/demos/vrrp-tracking.cast
100644 a9f501461c66c50a14c30c3d6e3f252e701e5051 0	assets/demos/vrrp-tracking.txt
100644 e92db6a0bfec648ffbaecfb891c174434fa56d1c 0	assets/demos/web-config.webp
100644 e698004d112edc9eeffead6a03d33f7833f0a08c 0	assets/product-dashboard.webp
100644 c9298247a37c695ace61fd1971aa2c3b3ebea731 0	assets/product-editor.webp
100644 a1fbe423e55fdbc0064378f8829ecf5643d801cc 0	assets/product-routes.webp
100644 4a0f020f62762060e96a6fedd58e32dee95d627b 0	assets/site.css
100644 e7fb6045947f34647a111a156769e1bc99cb7c6f 0	assets/site.js
100644 dedda7e1496b6b1c17e13eaae4438ec73cb0b354 0	demos/terminal/index.html
100644 d5d9f571d85d993b2e64b5430d3c1bc04a991eab 0	demos/terminal/index.md
100644 299c408a9b8047bc7b9ebdf36c14c0f01bbed1f7 0	style-guide/index.html
100644 bf3c57764629b61ae3c81f90fdfd518a2aac7b11 0	style-guide/index.md
100644 c17224f63cf088289cc745ef3b17b28bf5eaf061 0	index.html
100644 42ba3219dabe99c40f22d80a066fa1ea3085bcc5 0	index.md
100644 e9f958f151f4fa78e500cb9436d72501e9cef2da 0	features/web-interface/index.html
100644 3fbb4d06462eb54a90f374b86a59ac2895e4a5cf 0	features/web-interface/index.md
100644 c93991d8632d3d09e9648104fd501c86fbab62f0 0	guides/bfd/index.html
100644 0bac3344eb68a8eff5b60215cf1c542b07d69de9 0	guides/bfd/index.md
100644 b6fce3c873ac432f696fa7925059c48d2c426493 0	guides/config-editor/index.html
100644 c10216e2e96d67a34176ace74622f5a9c87ee2a1 0	guides/config-editor/index.md
100644 f3f184ec1d8a33a730c0506ceca278ef531a37b4 0	guides/ipsec/index.html
100644 0b093df971a3698346ce5195a17f1f7bd20f6634 0	guides/ipsec/index.md
100644 dd6086f774aae6aadb4fe60838b08593a84a33ee 0	guides/irr-filtering/index.html
100644 e62ccee9d082d00ebab03f30673f470100b76130 0	guides/irr-filtering/index.md
100644 e27a04dfa5d551a569de583e625b2565d2a66b0e 0	guides/ospf/index.html
100644 ee7aef4da9910915d47400f9bb4adf41acb5f4c7 0	guides/ospf/index.md
100644 15e027a3ee1026eeb3250debeb318be2377bccf2 0	guides/rpki/index.html
100644 74a1651fd0ad5c9532da48a54ec5870baa866892 0	guides/rpki/index.md
100644 9461ea68817d02d53021fe9e3e93b449ecb01d65 0	guides/traffic-usage/index.html
100644 8d4fd07e9da75bb9213892edf508ee4860bbb5d5 0	guides/traffic-usage/index.md
100644 01dbfca82a465076e89b54515902117eb9f2084f 0	guides/vrrp/index.html
100644 adb263c8f6e9de8adca0d7116109595f0e35e5a9 0	guides/vrrp/index.md
100644 3b3d93ccf25ce4877abc443c177bcf4ffa1d5cbc 0	data/search-index.json
ZE_INDEX_INFO
_ze_lost=$(git cat-file --batch-check='%(objectname)' <<'ZE_INDEX_INFO' | sed -n 's/ missing$//p'
38dfe1f36fe4cff87cc0932f457f265b5653d3da
bdef97f8449812114bd8de6422e9a5e10276e9aa
7302016eb5e14b1ea0c366fb209c605d41d424de
d65f7ccff4d0a8430da8f583899629b1332048ad
ea97cc698a9e6ae0676dc7a7eedd7d72ceab0207
632bb8932935e5987a8dde4f15409458cfa8b18f
bab468c25ac4da910296c1135fc992f9974c2bf9
d580a21dabae4e8f278a633dbaca920b3708805c
2173e9ba7304ffedd86efaf5acc535452905275e
382df12b5f68f7ab5780dd591bab5499681cb63e
194cb00ce3caaa465a0ba40566a5ce96ed275d3a
e46a93199885651cd87080f28b3b245ab4840a2d
f90e8fc82bba7587bda3e42654019a133f75aa5e
09385c0350a5be5b2cf126d358e4f237434a9b47
0ea3be00dbe3e3fe931d79ab0336866b44a91eec
0d4c0cb9800d559f17f23edf4d68f9c464526d83
90920c6f0c343eaab3c1b44ca7684b1c3b9d0440
d9df42db49447fb423d445949f3b413cc490be1f
a9f501461c66c50a14c30c3d6e3f252e701e5051
e92db6a0bfec648ffbaecfb891c174434fa56d1c
e698004d112edc9eeffead6a03d33f7833f0a08c
c9298247a37c695ace61fd1971aa2c3b3ebea731
a1fbe423e55fdbc0064378f8829ecf5643d801cc
4a0f020f62762060e96a6fedd58e32dee95d627b
e7fb6045947f34647a111a156769e1bc99cb7c6f
dedda7e1496b6b1c17e13eaae4438ec73cb0b354
d5d9f571d85d993b2e64b5430d3c1bc04a991eab
299c408a9b8047bc7b9ebdf36c14c0f01bbed1f7
bf3c57764629b61ae3c81f90fdfd518a2aac7b11
c17224f63cf088289cc745ef3b17b28bf5eaf061
42ba3219dabe99c40f22d80a066fa1ea3085bcc5
e9f958f151f4fa78e500cb9436d72501e9cef2da
3fbb4d06462eb54a90f374b86a59ac2895e4a5cf
c93991d8632d3d09e9648104fd501c86fbab62f0
0bac3344eb68a8eff5b60215cf1c542b07d69de9
b6fce3c873ac432f696fa7925059c48d2c426493
c10216e2e96d67a34176ace74622f5a9c87ee2a1
f3f184ec1d8a33a730c0506ceca278ef531a37b4
0b093df971a3698346ce5195a17f1f7bd20f6634
dd6086f774aae6aadb4fe60838b08593a84a33ee
e62ccee9d082d00ebab03f30673f470100b76130
e27a04dfa5d551a569de583e625b2565d2a66b0e
ee7aef4da9910915d47400f9bb4adf41acb5f4c7
15e027a3ee1026eeb3250debeb318be2377bccf2
74a1651fd0ad5c9532da48a54ec5870baa866892
9461ea68817d02d53021fe9e3e93b449ecb01d65
8d4fd07e9da75bb9213892edf508ee4860bbb5d5
01dbfca82a465076e89b54515902117eb9f2084f
adb263c8f6e9de8adca0d7116109595f0e35e5a9
3b3d93ccf25ce4877abc443c177bcf4ffa1d5cbc
ZE_INDEX_INFO
)
if [ -n "$_ze_lost" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "The content prepared for these paths is gone from the object store, collected by a git gc:" >&2
  GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 'index.html' 'index.md' 'features/web-interface/index.html' 'features/web-interface/index.md' 'guides/bfd/index.html' 'guides/bfd/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'guides/ipsec/index.html' 'guides/ipsec/index.md' 'guides/irr-filtering/index.html' 'guides/irr-filtering/index.md' 'guides/ospf/index.html' 'guides/ospf/index.md' 'guides/rpki/index.html' 'guides/rpki/index.md' 'guides/traffic-usage/index.html' 'guides/traffic-usage/index.md' 'guides/vrrp/index.html' 'guides/vrrp/index.md' 'data/search-index.json' | grep -F "$_ze_lost" | while read -r _ze_mode _ze_object _ze_stage _ze_path; do echo "invalid object $_ze_mode $_ze_object for '$_ze_path'"; done >&2
  echo "The working tree still holds each file. Prepare the commit again with ./le commit create." >&2
  exit 1
fi
_ze_removed=$(GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png')
GIT_INDEX_FILE="$_ze_index" git update-index --force-remove -- 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png'
_ze_tree=$(GIT_INDEX_FILE="$_ze_index" git write-tree)
_ze_status=0
_ze_commit=$(printf '%s\n' "$_ze_text" | git commit-tree ${_ze_sign:+-S} -p "$_ze_parent" "$_ze_tree") || _ze_status=$?
if [ "$_ze_status" -ne 0 ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "git commit-tree could not build the commit (its error is above). When commit.gpgsign is true, signing is the usual cause:" >&2
  echo "make gpg, or gpg.program, sign with user.signingKey, then run this script again." >&2
  exit 1
fi
_ze_changed=$(git diff-tree -r -z --no-renames --name-only "$_ze_parent" "$_ze_commit" | tr '\0' '\n')
if [ -z "$_ze_changed" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "HEAD already holds the prepared content of every path it names, so it has nothing to commit." >&2
  echo "Drop this block, or prepare the commit again from your working tree with ./le commit create." >&2
  exit 1
fi
_ze_stray=$(printf '%s\n' "$_ze_changed" | grep -v -x -F -e 'assets/site.css' -e 'assets/site.js' -e 'assets/demos/manifest.json' -e 'demos/terminal/index.html' -e 'demos/terminal/index.md' -e 'style-guide/index.html' -e 'style-guide/index.md' -e 'assets/demos/bfd-failover.cast' -e 'assets/demos/bfd-failover.txt' -e 'assets/demos/commit-confirmed.cast' -e 'assets/demos/commit-confirmed.txt' -e 'assets/demos/irr-filter.cast' -e 'assets/demos/irr-filter.txt' -e 'assets/demos/ospf-adjacency.cast' -e 'assets/demos/ospf-adjacency.txt' -e 'assets/demos/rpki.cast' -e 'assets/demos/rpki.txt' -e 'assets/demos/traffic-anomaly.cast' -e 'assets/demos/traffic-anomaly.txt' -e 'assets/demos/ipsec.cast' -e 'assets/demos/ipsec.txt' -e 'assets/demos/showcase.cast' -e 'assets/demos/showcase.txt' -e 'assets/demos/vrrp-tracking.cast' -e 'assets/demos/vrrp-tracking.txt' -e 'assets/demos/web-config.webp' -e 'assets/product-routes.webp' -e 'assets/product-editor.webp' -e 'assets/product-dashboard.webp' -e 'index.html' -e 'index.md' -e 'features/web-interface/index.html' -e 'features/web-interface/index.md' -e 'guides/bfd/index.html' -e 'guides/bfd/index.md' -e 'guides/config-editor/index.html' -e 'guides/config-editor/index.md' -e 'guides/ipsec/index.html' -e 'guides/ipsec/index.md' -e 'guides/irr-filtering/index.html' -e 'guides/irr-filtering/index.md' -e 'guides/ospf/index.html' -e 'guides/ospf/index.md' -e 'guides/rpki/index.html' -e 'guides/rpki/index.md' -e 'guides/traffic-usage/index.html' -e 'guides/traffic-usage/index.md' -e 'guides/vrrp/index.html' -e 'guides/vrrp/index.md' -e 'data/search-index.json' -e 'assets/demos/vrrp-failover.cast' -e 'assets/demos/vrrp-failover.txt' -e 'assets/demos/web-config.webm' -e 'assets/demos/web-config.png' -e 'assets/product-routes.png' -e 'assets/product-editor.jpg' -e 'assets/product-dashboard.png' || [ "$?" -eq 1 ])
if [ -n "$_ze_stray" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "Its tree changes paths the commit does not name, against its parent $_ze_parent:" >&2
  printf '%s\n' "$_ze_stray" >&2
  echo "Publishing it would revert or add work this commit does not own. This is a defect in the" >&2
  echo "generated script: report it with this script's path, then prepare the commit again with ./le commit create." >&2
  exit 1
fi
if _ze_swap=$(git update-ref -m 'commit: site: publish WebP screenshots and terminal-event playback' HEAD "$_ze_commit" "$_ze_parent" 2>&1); then
  _ze_own="${_ze_own:-} $_ze_commit"
  break
fi
_ze_now=$(git rev-parse --verify 'HEAD^{commit}')
_ze_lock=$(printf '%s\n' "$_ze_swap" | sed -n "s/.*Unable to create '\(.*\.lock\)'.*/\1/p")
if [ "$_ze_now" = "$_ze_parent" ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "git update-ref refused to move HEAD, which still names the parent the commit was built on:" >&2
  printf '%s\n' "$_ze_swap" >&2
  if [ -n "$_ze_lock" ]; then echo "Another git process holds $_ze_lock, or one that stopped left it behind. Run this script again once the other git command finishes; remove $_ze_lock only when no git process is running."; else echo "Fix what git names (a permission, for example), then run this script again."; fi >&2
  exit 1
fi
if [ "$_ze_attempt" -ge 5 ]; then
  echo 'ERROR: commit a (site: publish WebP screenshots and terminal-event playback) was not published, and HEAD is unchanged by it.' >&2
  echo "HEAD moved under it 5 times in a row while other commits landed." >&2
  echo "Run this script again once the other sessions' commits have landed." >&2
  exit 1
fi
_ze_attempt=$((_ze_attempt + 1))
echo "NOTE: HEAD moved while commit a was being published; building it again on the new HEAD." >&2
done
git -c core.quotePath=false show --stat --format='[%h] %s' "$_ze_commit"
rm -f "$_ze_index.now"
GIT_INDEX_FILE="$_ze_index.now" git add -f -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 2>/dev/null || true
_ze_drift=$({ GIT_INDEX_FILE="$_ze_index.now" git -c core.quotePath=false ls-files -s -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp'; GIT_INDEX_FILE="$_ze_index" git -c core.quotePath=false ls-files -s -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp'; } | sort | uniq -u | cut -f2- | sort -u)
rm -f "$_ze_index.now"
if [ -n "$_ze_drift" ]; then
  echo "NOTE: these paths changed on disk after this commit was prepared." >&2
  echo "The commit carries the prepared content; the difference stays in the working tree." >&2
  echo "$_ze_drift" >&2
fi
rm -f "$_ze_message"
# Delete each removed path's working-tree copy that git now holds byte for byte.
_ze_gone=('assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png')
rm -f "$_ze_index.gone"
for _ze_path in "${_ze_gone[@]}"; do
  if [ ! -e "$_ze_path" ] && [ ! -L "$_ze_path" ]; then continue; fi
  if [ -d "$_ze_path" ] && [ ! -L "$_ze_path" ]; then
    echo "NOTE: kept $_ze_path: a directory is not the content this commit removed." >&2
    continue
  fi
  GIT_INDEX_FILE="$_ze_index.gone" git --literal-pathspecs add -f -- "$_ze_path" 2>/dev/null || true
  _ze_now=$(GIT_INDEX_FILE="$_ze_index.gone" git --literal-pathspecs -c core.quotePath=false ls-files -s -- "$_ze_path")
  if [ -n "$_ze_now" ] && printf '%s\n' "$_ze_removed" | grep -q -x -F -- "$_ze_now"; then
    rm -f -- "$_ze_path"
  else
    echo "NOTE: kept $_ze_path: its working-tree copy is not the content this commit removed." >&2
  fi
done
rm -f "$_ze_index.gone"
# Point the shared index at what was committed. Nothing else in it is touched.
_ze_waited=0
until _ze_said=$({ git ls-tree HEAD -- 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 'index.html' 'index.md' 'features/web-interface/index.html' 'features/web-interface/index.md' 'guides/bfd/index.html' 'guides/bfd/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'guides/ipsec/index.html' 'guides/ipsec/index.md' 'guides/irr-filtering/index.html' 'guides/irr-filtering/index.md' 'guides/ospf/index.html' 'guides/ospf/index.md' 'guides/rpki/index.html' 'guides/rpki/index.md' 'guides/traffic-usage/index.html' 'guides/traffic-usage/index.md' 'guides/vrrp/index.html' 'guides/vrrp/index.md' 'data/search-index.json' | git update-index --index-info && git update-index --force-remove -- 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png'; } 2>&1); do
  case "$_ze_said" in
    *index.lock*)
      if [ "$_ze_waited" -lt 30 ]; then
        sleep 1
        _ze_waited=$((_ze_waited + 1))
        continue
      fi
      ;;
  esac
  echo "ERROR: commit $_ze_commit landed, but the shared index could not be pointed at it." >&2
  echo "git said: $_ze_said" >&2
  echo "These paths read as staged changes in every session until the repair runs:" >&2
  printf '%s\n' 'assets/site.css' 'assets/site.js' 'assets/demos/manifest.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'style-guide/index.html' 'style-guide/index.md' 'assets/demos/bfd-failover.cast' 'assets/demos/bfd-failover.txt' 'assets/demos/commit-confirmed.cast' 'assets/demos/commit-confirmed.txt' 'assets/demos/irr-filter.cast' 'assets/demos/irr-filter.txt' 'assets/demos/ospf-adjacency.cast' 'assets/demos/ospf-adjacency.txt' 'assets/demos/rpki.cast' 'assets/demos/rpki.txt' 'assets/demos/traffic-anomaly.cast' 'assets/demos/traffic-anomaly.txt' 'assets/demos/ipsec.cast' 'assets/demos/ipsec.txt' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'assets/demos/vrrp-tracking.cast' 'assets/demos/vrrp-tracking.txt' 'assets/demos/web-config.webp' 'assets/product-routes.webp' 'assets/product-editor.webp' 'assets/product-dashboard.webp' 'index.html' 'index.md' 'features/web-interface/index.html' 'features/web-interface/index.md' 'guides/bfd/index.html' 'guides/bfd/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'guides/ipsec/index.html' 'guides/ipsec/index.md' 'guides/irr-filtering/index.html' 'guides/irr-filtering/index.md' 'guides/ospf/index.html' 'guides/ospf/index.md' 'guides/rpki/index.html' 'guides/rpki/index.md' 'guides/traffic-usage/index.html' 'guides/traffic-usage/index.md' 'guides/vrrp/index.html' 'guides/vrrp/index.md' 'data/search-index.json' 'assets/demos/vrrp-failover.cast' 'assets/demos/vrrp-failover.txt' 'assets/demos/web-config.webm' 'assets/demos/web-config.png' 'assets/product-routes.png' 'assets/product-editor.jpg' 'assets/product-dashboard.png' >&2
  echo "Any later commit in this script did not run." >&2
  printf 'Run this from %s once .git/index.lock is released:\n' "$PWD" >&2
  echo 'git ls-tree HEAD -- '"'"'assets/site.css'"'"' '"'"'assets/site.js'"'"' '"'"'assets/demos/manifest.json'"'"' '"'"'demos/terminal/index.html'"'"' '"'"'demos/terminal/index.md'"'"' '"'"'style-guide/index.html'"'"' '"'"'style-guide/index.md'"'"' '"'"'assets/demos/bfd-failover.cast'"'"' '"'"'assets/demos/bfd-failover.txt'"'"' '"'"'assets/demos/commit-confirmed.cast'"'"' '"'"'assets/demos/commit-confirmed.txt'"'"' '"'"'assets/demos/irr-filter.cast'"'"' '"'"'assets/demos/irr-filter.txt'"'"' '"'"'assets/demos/ospf-adjacency.cast'"'"' '"'"'assets/demos/ospf-adjacency.txt'"'"' '"'"'assets/demos/rpki.cast'"'"' '"'"'assets/demos/rpki.txt'"'"' '"'"'assets/demos/traffic-anomaly.cast'"'"' '"'"'assets/demos/traffic-anomaly.txt'"'"' '"'"'assets/demos/ipsec.cast'"'"' '"'"'assets/demos/ipsec.txt'"'"' '"'"'assets/demos/showcase.cast'"'"' '"'"'assets/demos/showcase.txt'"'"' '"'"'assets/demos/vrrp-tracking.cast'"'"' '"'"'assets/demos/vrrp-tracking.txt'"'"' '"'"'assets/demos/web-config.webp'"'"' '"'"'assets/product-routes.webp'"'"' '"'"'assets/product-editor.webp'"'"' '"'"'assets/product-dashboard.webp'"'"' '"'"'index.html'"'"' '"'"'index.md'"'"' '"'"'features/web-interface/index.html'"'"' '"'"'features/web-interface/index.md'"'"' '"'"'guides/bfd/index.html'"'"' '"'"'guides/bfd/index.md'"'"' '"'"'guides/config-editor/index.html'"'"' '"'"'guides/config-editor/index.md'"'"' '"'"'guides/ipsec/index.html'"'"' '"'"'guides/ipsec/index.md'"'"' '"'"'guides/irr-filtering/index.html'"'"' '"'"'guides/irr-filtering/index.md'"'"' '"'"'guides/ospf/index.html'"'"' '"'"'guides/ospf/index.md'"'"' '"'"'guides/rpki/index.html'"'"' '"'"'guides/rpki/index.md'"'"' '"'"'guides/traffic-usage/index.html'"'"' '"'"'guides/traffic-usage/index.md'"'"' '"'"'guides/vrrp/index.html'"'"' '"'"'guides/vrrp/index.md'"'"' '"'"'data/search-index.json'"'"' | git update-index --index-info' >&2
  echo 'git update-index --force-remove -- '"'"'assets/demos/vrrp-failover.cast'"'"' '"'"'assets/demos/vrrp-failover.txt'"'"' '"'"'assets/demos/web-config.webm'"'"' '"'"'assets/demos/web-config.png'"'"' '"'"'assets/product-routes.png'"'"' '"'"'assets/product-editor.jpg'"'"' '"'"'assets/product-dashboard.png'"'"'' >&2
  echo 'rm -f '"'"'tmp/commit-408ec569-a-1743da.index'"'"'' >&2
  exit 1
done
rm -f 'tmp/commit-408ec569-a-1743da.index'
